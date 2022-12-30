package lz

import (
	"errors"
	"fmt"
	"math/bits"
	"reflect"
)

// hashSequencer allows the creation of sequence blocks using a simple hash
// table.
type hashSequencer struct {
	SeqBuffer

	hash
}

// MemSize returns the the memory that the HashSequencer occupies.
func (s *hashSequencer) MemSize() uintptr {
	n := reflect.TypeOf(*s).Size()
	n += s.SeqBuffer.additionalMemSize()
	n += s.hash.additionalMemSize()
	return n
}

// HSConfig provides the configuration parameters for the
// HashSequencer.
type HSConfig struct {
	SBConfig
	// number of bits of the hash index
	HashBits int
	// length of the input used; range [2,8]
	InputLen int
}

// ApplyDefaults sets values that are zero to their defaults values.
func (cfg *HSConfig) ApplyDefaults() {
	cfg.SBConfig.ApplyDefaults()
	if cfg.InputLen == 0 {
		cfg.InputLen = 3
	}
	if cfg.HashBits == 0 {
		cfg.HashBits = 12
	}
}

// Verify checks the config for correctness.
func (cfg *HSConfig) Verify() error {
	if err := cfg.SBConfig.Verify(); err != nil {
		return err
	}

	if !(2 <= cfg.InputLen && cfg.InputLen <= 8) {
		return fmt.Errorf(
			"lz: InputLen=%d; must be in range [2,8]", cfg.InputLen)
	}
	if !(cfg.InputLen <= cfg.WindowSize) {
		return fmt.Errorf(
			"lz: cfg.WindowSize is %d; must be >= InputLen=%d",
			cfg.WindowSize, cfg.InputLen)
	}
	if !(int64(cfg.WindowSize) <= int64(maxUint32)) {
		// We manage positions only as uint32 values and so this limit
		// is necessary
		return fmt.Errorf(
			"lz: WindowSize=%d; must be less than MaxUint32=%d",
			cfg.WindowSize, maxUint32)
	}
	maxHashBits := 24
	if t := 8 * cfg.InputLen; t < maxHashBits {
		maxHashBits = t
	}
	if !(0 <= cfg.HashBits && cfg.HashBits <= maxHashBits) {
		return fmt.Errorf("lz: HashBits=%d; must be <= %d",
			cfg.HashBits, maxHashBits)
	}
	return nil
}

// NewSequencer creates a new hash sequencer.
func (cfg HSConfig) NewSequencer() (s Sequencer, err error) {
	return newHashSequencer(cfg)
}

// newHashSequencer creates a new hash sequencer.
func newHashSequencer(cfg HSConfig) (s *hashSequencer, err error) {
	s = new(hashSequencer)
	if err := s.Init(cfg); err != nil {
		return nil, err
	}
	return s, nil
}

// Init initializes the hash sequencer. It returns an error if there is an issue
// with the configuration parameters.
func (s *hashSequencer) Init(cfg HSConfig) error {
	cfg.ApplyDefaults()
	var err error
	if err = cfg.Verify(); err != nil {
		return err
	}

	err = s.SeqBuffer.Init(cfg.SBConfig)
	if err != nil {
		return err
	}
	if err = s.hash.init(cfg.InputLen, cfg.HashBits); err != nil {
		return err
	}

	return nil
}

// Reset resets the hash sequencer. The sequencer will be in the same state as
// after Init.
func (s *hashSequencer) Reset(data []byte) error {
	if err := s.SeqBuffer.Reset(data); err != nil {
		return err
	}
	s.hash.reset()
	return nil
}

// Shrink shortens the window size to make more space available for Write and
// ReadFrom.
func (s *hashSequencer) Shrink() int {
	w := s.SeqBuffer.w
	n := s.SeqBuffer.shrink()
	s.hash.adapt(uint32(w - n))
	return n
}

func (s *hashSequencer) hashSegment(a, b int) {
	if a < 0 {
		a = 0
	}
	c := len(s.data) - s.inputLen + 1
	if b > c {
		b = c
	}

	// Ensure that we can use _getLE64 all the time.
	_p := s.data[:b+7]

	for i := a; i < b; i++ {
		x := _getLE64(_p[i:]) & s.mask
		s.table[s.hashValue(x)] = hashEntry{
			pos:   uint32(i),
			value: uint32(x),
		}
	}
}

// ErrEmptyBuffer indicates that the buffer is empty and no more data can be
// read or processed. More data must be provided to the buffer.
var ErrEmptyBuffer = errors.New("lz: empty buffer")

// Sequence converts the next block to sequences. The contents of the blk
// variable will be overwritten. The method returns the number of bytes
// sequenced and any error encountered. It return ErrEmptyBuffer if there is no
// further data available.
//
// If blk is nil the search structures will be filled. This mode can be used to
// ignore segments of data.
func (s *hashSequencer) Sequence(blk *Block, flags int) (n int, err error) {
	n = s.Buffered()
	if n > s.BlockSize {
		n = s.BlockSize
	}

	if blk == nil {
		if n == 0 {
			return 0, ErrEmptyBuffer
		}
		t := s.w + n
		s.hashSegment(s.w-s.hash.inputLen+1, t)
		s.w = t
		return n, nil

	}

	blk.Sequences = blk.Sequences[:0]
	blk.Literals = blk.Literals[:0]

	if n == 0 {
		return 0, ErrEmptyBuffer
	}

	s.hashSegment(s.w-s.inputLen+1, s.w)
	p := s.data[:s.w+n]

	inputEnd := len(p) - s.inputLen + 1
	i := s.w
	litIndex := i
	var minMatchLen int
	if s.inputLen < 3 {
		minMatchLen = s.inputLen
	} else {
		minMatchLen = 3
	}

	// Ensure that we can use _getLE64 all the time.
	_p := s.data[:inputEnd+7]

	for ; i < inputEnd; i++ {
		y := _getLE64(_p[i:])
		x := y & s.mask
		h := s.hashValue(x)
		entry := s.table[h]
		v := uint32(x)
		s.table[h] = hashEntry{
			pos:   uint32(i),
			value: v,
		}
		if v != entry.value {
			continue
		}
		// potential match
		j := int(entry.pos)
		o := i - j
		if !(0 < o && o <= s.WindowSize) {
			continue
		}
		k := bits.TrailingZeros64(_getLE64(_p[j:])^y) >> 3
		if k > len(p)-i {
			k = len(p) - i
		}
		if k < minMatchLen {
			continue
		}
		if k == 8 {
			r := p[j+8:]
			q := p[i+8:]
			for len(q) >= 8 {
				x := _getLE64(r) ^ _getLE64(q)
				b := bits.TrailingZeros64(x) >> 3
				k += b
				if b < 8 {
					goto match
				}
				r = r[8:]
				q = q[8:]
			}
			if len(q) > 0 {
				x := getLE64(r) ^ getLE64(q)
				b := bits.TrailingZeros64(x) >> 3
				if b > len(q) {
					b = len(q)
				}
				k += b
			}
		match:
		}

		q := p[litIndex:i]
		blk.Sequences = append(blk.Sequences,
			Seq{
				LitLen:   uint32(len(q)),
				MatchLen: uint32(k),
				Offset:   uint32(o),
			})
		blk.Literals = append(blk.Literals, q...)
		litIndex = i + k
		var b int
		if litIndex > inputEnd {
			b = inputEnd
		} else {
			b = litIndex
		}
		for j = i + 1; j < b; j++ {
			x := _getLE64(_p[j:]) & s.mask
			h := s.hashValue(x)
			s.table[h] = hashEntry{
				pos:   uint32(j),
				value: uint32(x),
			}
		}
		i = litIndex - 1
	}

	if flags&NoTrailingLiterals != 0 {
		i = litIndex
	} else {
		blk.Literals = append(blk.Literals, p[litIndex:]...)
		i = len(p)
	}
	n = i - s.w
	s.w = i
	return n, nil
}
