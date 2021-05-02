package lz

import (
	"fmt"
	"math"

	"github.com/ulikunitz/lz/suffix"
)

type GSASConfig struct {
	// maximal window size
	WindowSize int
	// size of the window if the buffer is shrinked
	ShrinkSize int
	// maximum size of the buffer
	MaxSize int
	// BlockSize: target size for a block
	BlockSize int
	// minimum match len
	MinMatchLen int
}

func (cfg *GSASConfig) Verify() error {
	if !(2 <= cfg.MinMatchLen) {
		return fmt.Errorf(
			"lz: MinMatchLen is %d; want >= 2",
			cfg.MinMatchLen)
	}
	if !(cfg.MinMatchLen <= cfg.WindowSize) {
		return fmt.Errorf(
			"lz: WindowSize is %d; must be >= MinMatchLen=%d",
			cfg.WindowSize, cfg.MinMatchLen)
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
	if !(int64(cfg.MaxSize) <= int64(math.MaxInt32)) {
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
	return nil
}

func (cfg *GSASConfig) ApplyDefaults() {
	if cfg.BlockSize == 0 {
		cfg.BlockSize = 128 * 1024
	}
	if cfg.WindowSize == 0 {
		cfg.WindowSize = 8 * 1024 * 1024
	}
	if cfg.MaxSize == 0 {
		cfg.MaxSize = 16 * 1024 * 1024
	}
	if cfg.MinMatchLen == 0 {
		cfg.MinMatchLen = 3
	}
}

type GreedySuffixArraySequencer struct {
	seqBuffer

	sa    []int32
	isa   []int32
	bits  bitset
	saPos int

	blockSize   int
	minMatchLen int
}

func NewGreedySuffixArraySeqeuncer(cfg GSASConfig) (s *GreedySuffixArraySequencer, err error) {
	s = new(GreedySuffixArraySequencer)
	err = s.Init(cfg)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (s *GreedySuffixArraySequencer) Init(cfg GSASConfig) error {
	cfg.ApplyDefaults()
	var err error
	if err = cfg.Verify(); err != nil {
		return err
	}
	err = s.seqBuffer.Init(cfg.WindowSize, cfg.MaxSize, cfg.ShrinkSize)
	if err != nil {
		return err
	}
	s.blockSize = cfg.BlockSize
	s.minMatchLen = cfg.MinMatchLen
	s.saPos = 0
	s.sa = s.sa[:0]
	s.isa = s.isa[:0]
	s.bits.clear()
	return nil
}

func (s *GreedySuffixArraySequencer) Reset() {
	s.seqBuffer.Reset()
	s.saPos = 0
	s.sa = s.sa[:0]
	s.isa = s.isa[:0]
	s.bits.clear()
}

func (s *GreedySuffixArraySequencer) Requested() int {
	r := s.blockSize - s.buffered()
	if r <= 0 {
		return 0
	}
	if s.available() < r {
		s.Shrink()
		s.saPos = 0
		s.sa = s.sa[:0]
		s.isa = s.isa[:0]
		s.bits.clear()
	}
	return s.available()
}

func (s *GreedySuffixArraySequencer) sort() {
	// Set the start of the array to the start of the window.
	s.saPos = s.w - s.windowSize
	if s.saPos < 0 {
		s.saPos = 0
	}
	n := len(s.data) - s.saPos
	if n > math.MaxInt32 {
		panic("n too large")
	}
	if n <= cap(s.sa) {
		s.sa = s.sa[:n]
	} else {
		s.sa = make([]int32, n)
	}
	suffix.Sort(s.data[s.saPos:], s.sa)
	if n <= cap(s.isa) {
		s.isa = s.isa[:n]
	} else {
		s.isa = make([]int32, n)
	}
	for i, j := range s.sa {
		s.isa[j] = int32(i)
	}
	s.bits.init(n)
	t := s.w - s.saPos
	for i := 0; i < t; i++ {
		s.bits.insert(int(s.isa[i]))
	}
}

func (s *GreedySuffixArraySequencer) Sequence(blk *Block, flags int) (n int, err error) {
	n = s.buffered()
	if blk == nil {
		if n == 0 {
			return 0, ErrEmptyBuffer
		}
		s.w += n
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
	i := s.w - s.saPos
	if i+n > len(s.sa) {
		s.sort()
		i = s.w - s.saPos
	}

	p := s.data[s.saPos : s.saPos+len(s.sa)]
	litIndex := i
	for i < len(p) {
		j := int(s.isa[i])
		s.bits.insert(j)
		k1, ok1 := s.bits.memberBefore(j)
		k2, ok2 := s.bits.memberAfter(j)
		var f, m int
		if ok1 {
			f = int(s.sa[k1])
			m = matchLen(p[f:], p[i:])
		}
		if ok2 {
			f2 := int(s.sa[k2])
			m2 := matchLen(p[f2:], p[i:])
			if m2 > m || (m2 == m && f2 > f) {
				f, m = f2, m2
			}
		}
		if m < s.minMatchLen {
			i++
			continue
		}
		q := p[litIndex:i]
		blk.Sequences = append(blk.Sequences,
			Seq{
				MatchLen: uint32(m),
				LitLen:   uint32(len(q)),
				Offset:   uint32(i - f),
			})
		blk.Literals = append(blk.Literals, q...)
		litIndex = i + m
		for i++; i < litIndex; i++ {
			s.bits.insert(int(s.isa[i]))
		}
	}

	if flags&NoTrailingLiterals != 0 {
		i = litIndex
	} else {
		blk.Literals = append(blk.Literals, p[litIndex:]...)
		i = len(p)
	}

	i += s.saPos
	n = i - s.w
	s.w = i
	return n, nil
}
