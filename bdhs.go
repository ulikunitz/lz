package lz

import (
	"fmt"
	"math/bits"
	"reflect"
)

// BDHSConfig provides the configuration parameters for the backward-looking
// double Hash Sequencer.
type BDHSConfig struct {
	// sequence buffer configuration
	SBConfig

	// smaller hash input length; range 2 to 8
	InputLen1 int
	// hash bits for the smaller hash input length
	HashBits1 int
	// larger input length; range 2 to 8
	InputLen2 int
	// hash bits for the larger hash input length
	HashBits2 int
}

// Verify checks the configuration for errors.
func (cfg *BDHSConfig) Verify() error {
	if err := cfg.SBConfig.Verify(); err != nil {
		return err
	}
	if !(2 <= cfg.InputLen1 && cfg.InputLen1 <= 8) {
		return fmt.Errorf(
			"lz: InputLen=%d; must be in range [2,8]",
			cfg.InputLen1)
	}
	if !(cfg.InputLen1 <= cfg.WindowSize) {
		return fmt.Errorf(
			"lz: cfg.WindowSize is %d;"+
				" must be >= InputLen=%d",
			cfg.WindowSize, cfg.InputLen1)
	}
	if !(int64(cfg.WindowSize) <= int64(maxUint32)) {
		// We manage positions only as uint32 values and so this limit
		// is necessary
		return fmt.Errorf(
			"lz: MaxSize=%d; must be less than MaxUint32=%d",
			cfg.WindowSize, maxUint32)
	}
	if !(cfg.InputLen1 < cfg.InputLen2 && cfg.InputLen2 <= 8) {
		return fmt.Errorf(
			"lz: cfg.InputLen2 is %d; must be in range [%d;%d]",
			cfg.InputLen2, cfg.InputLen1+1, 8)
	}

	maxHashBits1 := 24
	if t := 8 * cfg.InputLen1; t < maxHashBits1 {
		maxHashBits1 = t
	}
	if !(0 <= cfg.HashBits1 && cfg.HashBits1 <= maxHashBits1) {
		return fmt.Errorf("lz: HashBits1=%d; must be in range [%d,%d]",
			cfg.HashBits1, 0, maxHashBits1)
	}

	maxHashBits2 := 24
	if t := 8 * cfg.InputLen2; t < maxHashBits2 {
		maxHashBits2 = t
	}
	if !(0 <= cfg.HashBits2 && cfg.HashBits2 <= maxHashBits2) {
		return fmt.Errorf("lz: HashBits2=%d; must be in range [%d,%d]",
			cfg.HashBits2, 0, maxHashBits2)
	}

	return nil
}

// ApplyDefaults sets for the zero fields in the configuration to the default
// values.
func (cfg *BDHSConfig) ApplyDefaults() {
	cfg.SBConfig.ApplyDefaults()
	if cfg.InputLen1 == 0 {
		cfg.InputLen1 = 3
	}
	if cfg.HashBits1 == 0 {
		cfg.HashBits1 = 11
	}
	if cfg.InputLen2 == 0 {
		cfg.InputLen2 = 7
	}
	if cfg.HashBits2 == 0 {
		cfg.HashBits2 = 11
	}
}

// NewSequencer creates a new BackwardDoubleHashSequencer.
func (cfg BDHSConfig) NewSequencer() (s Sequencer, err error) {
	return newBackwardDoubleHashSequencer(cfg)
}

// backwardDoubleHashSequencer uses two hashes and tries to extend matches
// backward.
type backwardDoubleHashSequencer struct {
	SeqBuffer

	h1 hash
	h2 hash
}

// MemSize returns the consumed memory size by the sequencer.
func (s *backwardDoubleHashSequencer) MemSize() uintptr {
	n := reflect.TypeOf(*s).Size()
	n += s.SeqBuffer.additionalMemSize()
	n += s.h1.additionalMemSize()
	n += s.h2.additionalMemSize()
	return n
}

// newBackwardDoubleHashSequencer creates a new sequencer. If the configuration
// is invalid an error will be returned.
func newBackwardDoubleHashSequencer(cfg BDHSConfig) (s *backwardDoubleHashSequencer, err error) {
	s = new(backwardDoubleHashSequencer)
	if err = s.Init(cfg); err != nil {
		return nil, err
	}
	return s, nil
}

// Init initializes the sequencer. The method returns an error if the
// configuration contains inconsistencies and the sequencer remains
// uninitialized.
func (s *backwardDoubleHashSequencer) Init(cfg BDHSConfig) error {
	cfg.ApplyDefaults()
	var err error
	if err = cfg.Verify(); err != nil {
		return err
	}

	err = s.SeqBuffer.Init(cfg.SBConfig)
	if err != nil {
		return err
	}
	if err = s.h1.init(cfg.InputLen1, cfg.HashBits1); err != nil {
		return err
	}
	if err = s.h2.init(cfg.InputLen2, cfg.HashBits2); err != nil {
		return err
	}
	return nil
}

// Reset puts the sequencer into the state after initialization. The allocated
// memory in the buffer will be maintained.
func (s *backwardDoubleHashSequencer) Reset(data []byte) error {
	if err := s.SeqBuffer.Reset(data); err != nil {
		return err
	}
	s.h1.reset()
	s.h2.reset()
	return nil
}

func (s *backwardDoubleHashSequencer) hashSegment1(a, b int) {
	if a < 0 {
		a = 0
	}
	e1 := len(s.data) - s.h1.inputLen + 1
	if b < e1 {
		e1 = b
	}

	_p := s.data[:e1+7]

	for i := a; i < e1; i++ {
		x := _getLE64(_p[i:]) & s.h1.mask
		h := hashValue(x, s.h1.shift)
		s.h1.table[h] = hashEntry{
			pos:   uint32(i),
			value: uint32(x),
		}
	}
}

func (s *backwardDoubleHashSequencer) hashSegment2(a, b int) {
	if a < 0 {
		a = 0
	}
	e2 := len(s.data) - s.h2.inputLen + 1
	if b < e2 {
		e2 = b
	}

	_p := s.data[:e2+7]

	for i := a; i < e2; i++ {
		x := _getLE64(_p[i:]) & s.h2.mask
		h := hashValue(x, s.h2.shift)
		s.h2.table[h] = hashEntry{
			pos:   uint32(i),
			value: uint32(x),
		}
	}
}

// Sequence computes the LZ77 sequence for the next block. It returns the number
// of bytes actually sequenced. ErrEmptyBuffer will be returned if there is no
// data to sequence.
func (s *backwardDoubleHashSequencer) Sequence(blk *Block, flags int) (n int, err error) {
	n = s.Buffered()
	if n > s.BlockSize {
		n = s.BlockSize
	}

	if blk == nil {
		if n == 0 {
			return 0, ErrEmptyBuffer
		}
		t := s.w + n
		s.hashSegment1(s.w-s.h1.inputLen+1, t)
		s.hashSegment2(s.w-s.h2.inputLen+1, t)
		s.w = t
		return n, nil
	}

	blk.Sequences = blk.Sequences[:0]
	blk.Literals = blk.Literals[:0]

	if n == 0 {
		return 0, ErrEmptyBuffer
	}

	s.hashSegment1(s.w-s.h1.inputLen+1, s.w)
	s.hashSegment2(s.w-s.h2.inputLen+1, s.w)
	p := s.data[:s.w+n]

	e1 := len(p) - s.h1.inputLen + 1
	e2 := len(p) - s.h2.inputLen + 1
	i := s.w
	litIndex := i

	minMatchLen := 3
	if s.h1.inputLen < minMatchLen {
		minMatchLen = s.h1.inputLen
	}

	// Ensure that we can use _getLE64 all the time.
	_p := s.data[:e1+7]

	for ; i < e2; i++ {
		y := _getLE64(_p[i:])
		x := y & s.h2.mask
		h := hashValue(x, s.h2.shift)
		entry := s.h2.table[h]
		v2 := uint32(x)
		pos := uint32(i)
		s.h2.table[h] = hashEntry{pos: pos, value: v2}
		x = y & s.h1.mask
		h = hashValue(x, s.h1.shift)
		entry1 := s.h1.table[h]
		v1 := uint32(x)
		s.h1.table[h] = hashEntry{pos: pos, value: v1}
		if v2 != entry.value {
			if v1 != entry1.value {
				continue
			}
			entry = entry1
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
		if back := i - litIndex; back > 0 {
			if back > j {
				back = j
			}
			m := backwardMatchLen(p[j-back:j], p[:i])
			i -= m
			k += m
		}
		q := p[litIndex:i]
		blk.Sequences = append(blk.Sequences,
			Seq{
				MatchLen: uint32(k),
				LitLen:   uint32(len(q)),
				Offset:   uint32(o),
			})
		blk.Literals = append(blk.Literals, q...)
		litIndex = i + k
		b := litIndex
		if litIndex > e2 {
			b = e2
		}
		for j = i + 1; j < b; j++ {
			y := _getLE64(_p[j:])
			x := y & s.h2.mask
			h := hashValue(x, s.h2.shift)
			pos := uint32(j)
			s.h2.table[h] = hashEntry{pos: pos, value: uint32(x)}
			x = y & s.h1.mask
			h = hashValue(x, s.h1.shift)
			s.h1.table[h] = hashEntry{pos: pos, value: uint32(x)}
		}
		if j < litIndex {
			b = litIndex
			if litIndex > e1 {
				b = e1
			}
			for ; j < b; j++ {
				x := _getLE64(_p[j:]) & s.h1.mask
				h := hashValue(x, s.h1.shift)
				s.h1.table[h] = hashEntry{
					pos:   uint32(j),
					value: uint32(x),
				}
			}
		}
		i = litIndex - 1
	}
	for ; i < e1; i++ {
		y := _getLE64(_p[i:])
		x := y & s.h1.mask
		h := hashValue(x, s.h1.shift)
		entry := s.h1.table[h]
		v1 := uint32(x)
		s.h1.table[h] = hashEntry{
			pos:   uint32(i),
			value: v1,
		}
		if v1 != entry.value {
			continue
		}
		// potential match
		j := int(entry.pos)
		// j must not be less than window start
		o := i - j
		if !(0 < o && o <= s.WindowSize) {
			continue
		}
		k := bits.TrailingZeros64(_getLE64(_p[j:])^y) >> 3
		if k > len(p)-int(i) {
			k = len(p) - int(i)
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
					goto match1
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
		match1:
		}
		if back := i - litIndex; back > 0 {
			if back > j {
				back = j
			}
			m := backwardMatchLen(p[j-back:j], p[:i])
			i -= m
			k += m
		}
		q := p[litIndex:i]
		blk.Sequences = append(blk.Sequences,
			Seq{
				MatchLen: uint32(k),
				LitLen:   uint32(len(q)),
				Offset:   uint32(o),
			})
		blk.Literals = append(blk.Literals, q...)
		litIndex = i + k
		b := litIndex
		if b > e1 {
			b = e1
		}
		for ; j < b; j++ {
			x := _getLE64(_p[j:]) & s.h1.mask
			h := hashValue(x, s.h1.shift)
			s.h1.table[h] = hashEntry{
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
	n = int(i) - s.w
	s.w = int(i)
	return n, nil
}

// Shrink shortens the window size to make more space available for Write and
// ReadFrom.
func (s *backwardDoubleHashSequencer) Shrink() {
	delta := uint32(s.SeqBuffer.shrink())
	s.h1.Adapt(delta)
	s.h2.Adapt(delta)
}
