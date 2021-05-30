package lz

import (
	"fmt"
	"math/bits"
)

// DHSConfig provides the confifuration parameters for the DoubleHashSequencer.
type DHSConfig struct {
	// maximal window size
	WindowSize int
	// size of the window if the buffer is shrinked
	ShrinkSize int
	// maximum size of the buffer
	MaxSize int
	// BlockSize: target size for a block
	BlockSize int
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
func (cfg *DHSConfig) Verify() error {
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
			"lz: shrinkSize must be less than cfg.MaxSize")
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
	if !(cfg.InputLen1 < cfg.InputLen2 && cfg.InputLen2 <= 8) {
		return fmt.Errorf(
			"lz: cfg.InputLen2 is %d; must be in range [%d;%d]",
			cfg.InputLen2, cfg.InputLen1+1, 8)
	}

	maxHashBits1 := 32
	if t := 8 * cfg.InputLen1; t < maxHashBits1 {
		maxHashBits1 = t
	}
	if !(0 <= cfg.HashBits1 && cfg.HashBits1 <= maxHashBits1) {
		return fmt.Errorf("lz: HashBits1=%d; must be in range [%d,%d]",
			cfg.HashBits1, 0, maxHashBits1)
	}

	maxHashBits2 := 32
	if t := 8 * cfg.InputLen2; t < maxHashBits2 {
		maxHashBits2 = t
	}
	if !(0 <= cfg.HashBits2 && cfg.HashBits2 <= maxHashBits2) {
		return fmt.Errorf("lz: HashBits2=%d; must be in range [%d,%d]",
			cfg.HashBits2, 0, maxHashBits2)
	}

	return nil
}

// ApplyDefaults uses the defaults for the configuration parameters that are set
// to zero.
func (cfg *DHSConfig) ApplyDefaults() {
	if cfg.BlockSize == 0 {
		cfg.BlockSize = 128 * 1024
	}
	if cfg.WindowSize == 0 {
		cfg.WindowSize = 8 * 1024 * 1024
	}
	if cfg.MaxSize == 0 {
		cfg.MaxSize = 16 * 1024 * 1024
	}
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

// DoubleHashSequencer generates LZ77 sequences by using two hash tables. The
// input length for the two hash tables will be different. The speed of the hash
// sequencer is slower than sequencers using a single hash, but the compression
// ratio is much better.
type DoubleHashSequencer struct {
	seqBuffer

	h1 hash

	h2 hash

	pos uint32

	blockSize int
}

// NewDoubleHashSequencer allocates a new DoubleHashSequencer value and
// initializes it. The function returns the first error found in the
// configuration.
func NewDoubleHashSequencer(cfg DHSConfig) (s *DoubleHashSequencer, err error) {
	s = new(DoubleHashSequencer)
	if err = s.Init(cfg); err != nil {
		return nil, err
	}
	return s, nil
}

// Init initializes the DoubleHashSequencer. The first error found in the
// configuration will be returned.
func (s *DoubleHashSequencer) Init(cfg DHSConfig) error {
	cfg.ApplyDefaults()
	var err error
	if err = cfg.Verify(); err != nil {
		return err
	}

	err = s.seqBuffer.Init(cfg.WindowSize, cfg.MaxSize, cfg.ShrinkSize)
	if err != nil {
		return err
	}
	if err = s.h1.init(cfg.InputLen1, cfg.HashBits1); err != nil {
		return err
	}
	if err = s.h2.init(cfg.InputLen2, cfg.HashBits2); err != nil {
		return err
	}
	s.blockSize = cfg.BlockSize
	s.pos = 0
	return nil
}

// Reset puts the DoubleHashSequencer in its initial state.
func (s *DoubleHashSequencer) Reset() {
	s.seqBuffer.Reset()
	s.h1.reset()
	s.h2.reset()
	s.pos = 0
}

// Requested answers the question whether data needs to be provided to the
// sequencer. If no data is need 0 will be returned and otherwise the number of
// bytes that can be added to the internal buffer of the sequencer.
func (s *DoubleHashSequencer) Requested() int {
	r := s.blockSize - s.buffered()
	if r <= 0 {
		return 0
	}
	if s.available() < r {
		s.pos += uint32(s.Shrink())
		if int64(s.pos)+int64(s.max) > maxUint32 {
			s.h1.adapt(s.pos)
			s.h2.adapt(s.pos)
			s.pos = 0
		}
	}
	return s.available()
}

// hashSegment1 hases the provided segment of data for the first hash table.
func (s *DoubleHashSequencer) hashSegment1(a, b int) {
	if a < 0 {
		a = 0
	}
	n := len(s.data)
	e1 := n - s.h1.inputLen + 1
	if b < e1 {
		e1 = b
	}

	k := e1 + 8
	if k > cap(s.data) {
		var z [8]byte
		s.data = append(s.data, z[:k-n]...)[:n]
	}
	_p := s.data[:k]

	for i := a; i < e1; i++ {
		x := _getLE64(_p[i:]) & s.h1.mask
		h := s.h1.hashValue(x)
		s.h1.table[h] = hashEntry{
			pos:   s.pos + uint32(i),
			value: uint32(x),
		}
	}
}

// hashSegment computes the hashes for the second hash table.
func (s *DoubleHashSequencer) hashSegment2(a, b int) {
	if a < 0 {
		a = 0
	}
	n := len(s.data)
	e2 := n - s.h2.inputLen + 1
	if b < e2 {
		e2 = b
	}

	k := e2 + 8
	if k > cap(s.data) {
		var z [8]byte
		s.data = append(s.data, z[:k-n]...)[:n]
	}
	_p := s.data[:k]

	for i := a; i < e2; i++ {
		x := _getLE64(_p[i:]) & s.h2.mask
		h := s.h2.hashValue(x)
		s.h2.table[h] = hashEntry{
			pos:   s.pos + uint32(i),
			value: uint32(x),
		}
	}
}

// Sequence generates the LZ77 sequences. It returns the number of bytes covered
// by the new sequences. The block will be overwritten but the memory for the
// slices will be reused.
func (s *DoubleHashSequencer) Sequence(blk *Block, flags int) (n int, err error) {
	n = s.buffered()
	if blk == nil {
		if n == 0 {
			return 0, ErrEmptyBuffer
		}
		// TODO: we need to iterate over the segment only once
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
	if n > s.blockSize {
		n = s.blockSize
	}

	// TODO: we need to iterate over the segment only once
	s.hashSegment1(s.w-s.h1.inputLen+1, s.w)
	s.hashSegment2(s.w-s.h2.inputLen+1, s.w)
	p := s.data[:s.w+n]

	e1 := int64(len(p) - s.h1.inputLen + 1)
	e2 := int64(len(p) - s.h2.inputLen + 2)
	i := int64(s.w)
	litIndex := i

	// Ensure that we can use _getLE64 all the time.
	k := int(e1 + 8)
	if k > cap(s.data) {
		var z [8]byte
		m := len(s.data)
		s.data = append(s.data, z[:k-m]...)[:m]
	}
	_p := s.data[:k]

	for ; i < e2; i++ {
		y := _getLE64(_p[i:])
		x := y & s.h2.mask
		h := s.h2.hashValue(x)
		entry := s.h2.table[h]
		v2 := uint32(x)
		pos := s.pos + uint32(i)
		s.h2.table[h] = hashEntry{pos: pos, value: v2}
		x = y & s.h1.mask
		h = s.h1.hashValue(x)
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
		if litIndex > e2 {
			b = e2
		}
		for j = i + 1; j < b; j++ {
			y := _getLE64(_p[j:])
			x := y & s.h2.mask
			h := s.h2.hashValue(x)
			pos := s.pos + uint32(j)
			s.h2.table[h] = hashEntry{pos: pos, value: uint32(x)}
			x = y & s.h1.mask
			h = s.h1.hashValue(x)
			s.h1.table[h] = hashEntry{pos: pos, value: uint32(x)}
		}
		if j < litIndex {
			b = litIndex
			if litIndex > e1 {
				b = e1
			}
			for ; j < b; j++ {
				x := _getLE64(_p[j:]) & s.h1.mask
				h := s.h1.hashValue(x)
				s.h1.table[h] = hashEntry{
					pos:   s.pos + uint32(j),
					value: uint32(x),
				}
			}
		}
		i = litIndex - 1
	}
	for ; i < e1; i++ {
		y := _getLE64(_p[i:])
		x := y & s.h1.mask
		h := s.h1.hashValue(x)
		entry := s.h1.table[h]
		v1 := uint32(x)
		s.h1.table[h] = hashEntry{
			pos:   s.pos + uint32(i),
			value: v1,
		}
		if v1 != entry.value {
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
		if b > e1 {
			b = e1
		}
		for ; j < b; j++ {
			x := _getLE64(_p[j:]) & s.h1.mask
			h := s.h1.hashValue(x)
			s.h1.table[h] = hashEntry{
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