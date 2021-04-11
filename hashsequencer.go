package lz

import (
	"errors"
	"fmt"
)

type hashEntry struct {
	pos   uint32
	value uint32
}

type HashSequencer struct {
	seqWindow

	hashTable []hashEntry

	mask uint64
	// shift provides the shift required for the hash function
	shift uint

	inputLen    int
	minMatchLen int
}

const prime = 9920624304325388887

// hashes the masked x
func (s *HashSequencer) hash(x uint64) uint32 {
	return uint32((x * prime) >> s.shift)
}

// HashSequencerConfig provides the configuration parameters for the
// HashSequencer. The SeqWindow has a linear buffer that needs to be shrinked
// by sliding the window into the front of the buffer. The remaining size of the
// window after sliding the window is given by shrinkSize.
type HashSequencerConfig struct {
	// maximal window size
	WindowSize int
	// size of the window if the buffer is shrinked
	ShrinkSize int
	// maximum size of the buffer
	MaxSize int
	// number of bits of the hash index
	HashBits int
	// lenght of the input used; range [2,8]
	InputLen int
	// minimum match length; must be greater or equal 2
	MinMatchLen int
}

func (cfg *HashSequencerConfig) ApplyDefaults() {
	if cfg.WindowSize == 0 {
		cfg.WindowSize = 8 * 1024 * 1024
		if cfg.ShrinkSize > cfg.WindowSize {
			cfg.ShrinkSize = cfg.WindowSize
		}
		if cfg.MaxSize != 0 && cfg.MaxSize < cfg.WindowSize {
			cfg.MaxSize = cfg.WindowSize
		}
	}
	if cfg.MaxSize == 0 {
		cfg.MaxSize = 8 * 1024 * 1024
		if cfg.MaxSize < cfg.WindowSize {
			cfg.MaxSize = cfg.WindowSize
		}
		if cfg.MaxSize < cfg.ShrinkSize {
			cfg.MaxSize = 2 * cfg.ShrinkSize
		}
	}
	if cfg.MinMatchLen == 0 {
		cfg.MinMatchLen = 3
	}
	if cfg.InputLen == 0 {
		cfg.InputLen = 4
	}
	if cfg.HashBits == 0 {
		cfg.HashBits = 12
	}
}

func (cfg *HashSequencerConfig) Verify() error {
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
			"lz: WindowSize=&%d; must be less than MaxSize=%d",
			cfg.WindowSize, cfg.MaxSize)
	}
	maxHashBits := 32
	if t := 8 * cfg.InputLen; t < maxHashBits {
		maxHashBits = t

	}
	if !(2 <= cfg.MinMatchLen) {
		return fmt.Errorf(
			"lz: MinMatchLen=%d; must be >= 2", cfg.MinMatchLen)
	}
	if !(0 <= cfg.HashBits && cfg.HashBits <= maxHashBits) {
		return fmt.Errorf("lz: HashBits=%d; must be less than %d",
			cfg.HashBits, maxHashBits)
	}
	return nil
}

func (s *HashSequencer) Init(cfg HashSequencerConfig) error {
	cfg.ApplyDefaults()
	var err error
	if err = cfg.Verify(); err != nil {
		return err
	}
	err = s.seqWindow.init(cfg.WindowSize, cfg.MaxSize, cfg.ShrinkSize)
	if err != nil {
		return err
	}

	n := 1 << cfg.HashBits
	if n <= cap(s.hashTable) {
		s.hashTable = s.hashTable[:n]
	} else {
		s.hashTable = make([]hashEntry, n)
	}

	s.mask = 1<<(uint64(cfg.InputLen)*8) - 1
	s.shift = 64 - uint(cfg.HashBits)

	s.inputLen = cfg.InputLen
	s.minMatchLen = cfg.MinMatchLen
	return nil
}

var ErrEmptyBuffer = errors.New("lz: empty buffer")

func (s *HashSequencer) hashSegment(a, b int) {
	p := s.data
	if a < 0 {
		a = 0
	}
	c := len(p) - s.inputLen + 1
	if b > c {
		b = c
	}
	c = len(p) - 8 + 1
	if b <= c {
		c = b
	}
	for i := a; i < c; i++ {
		x := _getLE64(p[i:]) & s.mask
		h := s.hash(x)
		s.hashTable[h] = hashEntry{
			pos:   s.pos + uint32(i),
			value: uint32(x),
		}
	}
	for i := c; i < b; i++ {
		x := getLE64(p[i:]) & s.mask
		h := s.hash(x)
		s.hashTable[h] = hashEntry{
			pos:   s.pos + uint32(i),
			value: uint32(x),
		}
	}
}

func (s *HashSequencer) Sequence(blk *Block, k, flags int) (n int, err error) {
	n = k
	buffered := s.buffered()
	if n > buffered {
		n = buffered
	}
	if blk == nil {
		t := s.w + n
		s.hashSegment(s.w-s.inputLen+1, t)
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

	inputEnd := int64(len(p) - s.inputLen + 1)
	i := int64(s.w)
	litIndex := i
	for i < inputEnd {
		x := getLE64(p[i:]) & s.mask
		h := s.hash(x)
		entry := s.hashTable[h]
		v := uint32(x)
		s.hashTable[h] = hashEntry{
			pos:   s.pos + uint32(i),
			value: v,
		}
		if v != entry.value {
			i++
			continue
		}
		// potential match
		j := int64(entry.pos) - int64(s.pos)
		o := i - j
		if !(0 < o && o < int64(s.size)) {
			i++
			continue
		}
		k := matchLen(p[j:], p[i:])
		if k < s.minMatchLen {
			i++
			continue
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
		s.hashSegment(int(i+1), int(litIndex))
		i = litIndex
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
