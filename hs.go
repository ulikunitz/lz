package lz

import (
	"errors"
	"fmt"
	"math/bits"
	"reflect"
)

const maxUint32 = 1<<32 - 1

// HashSequencer allows the creation of sequence blocks using a simple hash
// table.
type HashSequencer struct {
	seqBuffer

	hash

	// pos of the start of data in Buffer
	pos uint32

	blockSize int
}

func (s *HashSequencer) MemSize() uintptr {
	n := reflect.TypeOf(*s).Size()
	n += s.seqBuffer.additionalMemSize()
	n += s.hash.additionalMemSize()
	return n
}

// BlockSize returns the block size supported by the sequencer.
func (s *HashSequencer) BlockSize() int { return s.blockSize }

// HSConfig provides the configuration parameters for the
// HashSequencer.
//
// The pos-buffer contains the sliding window. If the window reaches the end of
// the buffer parts of it needs to be moved to the front of the buffer. The
// number of bytes to be moved are defined by the shrinkSize. A shrinkSize of 0
// is supported.
type HSConfig struct {
	// maximal window size
	WindowSize int
	// size of the window if the buffer is shrinked
	ShrinkSize int
	// maximum size of the buffer
	MaxSize int
	// BlockSize: target size for a block
	BlockSize int
	// number of bits of the hash index
	HashBits int
	// length of the input used; range [2,8]
	InputLen int
}

// ApplyDefaults sets values that are zero to their defaults values.
func (cfg *HSConfig) ApplyDefaults() {
	if cfg.BlockSize == 0 {
		cfg.BlockSize = 128 * 1024
	}
	if cfg.WindowSize == 0 {
		cfg.WindowSize = 8 * 1024 * 1024
	}
	if cfg.MaxSize == 0 {
		cfg.MaxSize = 16 * 1024 * 1024
	}
	if cfg.InputLen == 0 {
		cfg.InputLen = 3
	}
	if cfg.HashBits == 0 {
		cfg.HashBits = 12
	}
}

// Verify checks the config for correctness.
func (cfg *HSConfig) Verify() error {
	if !(2 <= cfg.InputLen && cfg.InputLen <= 8) {
		return fmt.Errorf(
			"lz: InputLen=%d; must be in range [2,8]", cfg.InputLen)
	}
	if !(cfg.InputLen <= cfg.WindowSize) {
		return fmt.Errorf(
			"lz: cfg.WindowSize is %d; must be >= InputLen=%d",
			cfg.WindowSize, cfg.InputLen)
	}
	if !(0 <= cfg.ShrinkSize && cfg.ShrinkSize <= cfg.WindowSize) {
		return fmt.Errorf(
			"lz: ShrinkSize=%d; must be <= WindowSize=%d",
			cfg.ShrinkSize, cfg.WindowSize)
	}
	if !(cfg.WindowSize <= cfg.MaxSize) {
		return fmt.Errorf(
			"lz: WindowSize=%d; must be less than MaxSize=%d",
			cfg.WindowSize, cfg.MaxSize)
	}
	if !(cfg.ShrinkSize < cfg.MaxSize) {
		return fmt.Errorf(
			"ls: shrinkSize must be less than cfg.MaxSize")
	}
	if !(int64(cfg.MaxSize) <= int64(maxUint32)) {
		// We manage positions only as uint32 values and so this limit
		// is necessary
		return fmt.Errorf(
			"lz: MaxSize=%d; must be less than MaxUint32=%d",
			cfg.MaxSize, maxUint32)
	}
	if !(0 < cfg.BlockSize) {
		return fmt.Errorf(
			"lz: BlockSize=%d; must be positive", cfg.BlockSize)
	}
	maxHashBits := 32
	if t := 8 * cfg.InputLen; t < maxHashBits {
		maxHashBits = t
	}
	if !(0 <= cfg.HashBits && cfg.HashBits <= maxHashBits) {
		return fmt.Errorf("lz: HashBits=%d; must be <= %d",
			cfg.HashBits, maxHashBits)
	}
	return nil
}

// NewWriteSequencer creates a new hash sequencer.
func (cfg HSConfig) NewWriteSequencer() (s WriteSequencer, err error) {
	return NewHashSequencer(cfg)
}

// NewHashSequencer creates a new hash sequencer.
func NewHashSequencer(cfg HSConfig) (s *HashSequencer, err error) {
	var t HashSequencer
	if err := t.Init(cfg); err != nil {
		return nil, err
	}
	return &t, nil
}

// Init initialzes the hash sequencer. It returns an error if there is an issue
// with the configuration parameters.
func (s *HashSequencer) Init(cfg HSConfig) error {
	cfg.ApplyDefaults()
	var err error
	if err = cfg.Verify(); err != nil {
		return err
	}

	err = s.seqBuffer.Init(cfg.WindowSize, cfg.MaxSize, cfg.ShrinkSize)
	if err != nil {
		return err
	}
	if err = s.hash.init(cfg.InputLen, cfg.HashBits); err != nil {
		return err
	}

	s.blockSize = cfg.BlockSize
	s.pos = 0
	return nil
}

// Reset resets the hash sequencer. The sequencer will be in the same state as
// after Init.
func (s *HashSequencer) Reset() {
	s.seqBuffer.Reset()
	s.hash.reset()
	s.pos = 0
}

// Requested provides the number of bytes that the sequencer requests to be
// provided.
func (s *HashSequencer) Requested() int {
	r := s.blockSize - s.buffered()
	if r <= 0 {
		return 0
	}
	if s.available() < r {
		s.pos += uint32(s.Shrink())
		if int64(s.pos)+int64(s.max) > maxUint32 {
			s.adapt(s.pos)
			s.pos = 0
		}
	}
	return s.available()
}

func (s *HashSequencer) hashSegment(a, b int) {
	if a < 0 {
		a = 0
	}
	n := len(s.data)
	c := n - s.inputLen + 1
	if b > c {
		b = c
	}

	// Ensure that we can use _getLE64 all the time.
	k := b + 8
	if k > cap(s.data) {
		z := make([]byte, len(s.data), k)
		copy(z, s.data)
		s.data = z
	}
	_p := s.data[:k]

	for i := a; i < b; i++ {
		x := _getLE64(_p[i:]) & s.mask
		h := s.hashValue(x)
		s.table[h] = hashEntry{
			pos:   s.pos + uint32(i),
			value: uint32(x),
		}
	}
}

// ErrEmptyBuffer indicates that the buffer is empty and no more data can be
// read or processed. More data must be provided to the buffer.
var ErrEmptyBuffer = errors.New("lz: empty buffer")

// Sequence converts the next block of k bytes to a sequences. The block will be
// overwritten. The method returns the number of bytes sequenced and any error
// encountered. It return ErrEmptyBuffer if there is no further data available.
//
// If blk is nil the search structures will be filled. This mode can be used to
// ignore segments of data.
func (s *HashSequencer) Sequence(blk *Block, flags int) (n int, err error) {
	n = s.buffered()
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
	if n > s.blockSize {
		n = s.blockSize
	}

	s.hashSegment(s.w-s.inputLen+1, s.w)
	p := s.data[:s.w+n]

	inputEnd := int64(len(p) - s.inputLen + 1)
	i := int64(s.w)
	litIndex := i

	// Ensure that we can use _getLE64 all the time.
	k := int(inputEnd + 8)
	if k > cap(s.data) {
		z := make([]byte, len(s.data), k)
		copy(z, s.data)
		s.data = z
	}
	_p := s.data[:k]

	for ; i < inputEnd; i++ {
		y := _getLE64(_p[i:])
		x := y & s.mask
		h := s.hashValue(x)
		entry := s.table[h]
		v := uint32(x)
		s.table[h] = hashEntry{
			pos:   s.pos + uint32(i),
			value: v,
		}
		if v != entry.value {
			continue
		}
		// potential match
		j := int64(entry.pos) - int64(s.pos)
		// j must not be less than window start
		if j < doz64(i, int64(s.windowSize)) {
			continue
		}
		o := i - j
		if o <= 0 {
			continue
		}
		k := bits.TrailingZeros64(_getLE64(_p[j:])^y) >> 3
		if k > len(p)-int(i) {
			k = len(p) - int(i)
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
				MatchLen: uint32(k),
				LitLen:   uint32(len(q)),
				Offset:   uint32(o),
			})
		blk.Literals = append(blk.Literals, q...)
		litIndex = i + int64(k)
		b := litIndex
		if litIndex > inputEnd {
			b = inputEnd
		}
		for j = i + 1; j < b; j++ {
			x := _getLE64(_p[j:]) & s.mask
			h := s.hashValue(x)
			s.table[h] = hashEntry{
				pos:   s.pos + uint32(j),
				value: uint32(x),
			}
		}
		i = litIndex - 1
	}

	if flags&NoTrailingLiterals != 0 {
		i = litIndex
	} else {
		blk.Literals = append(blk.Literals, p[litIndex:]...)
		i = int64(len(p))
	}
	n = int(i) - s.w
	s.w = int(i)
	return n, nil
}
