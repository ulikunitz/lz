package lz

import (
	"fmt"
	"math"
	"math/bits"

	"github.com/ulikunitz/lz/suffix"
)

// OSASConfig defines the configuration parameter for the optimal suffix array
// seqeuncer.
type OSASConfig struct {
	// maximal window size
	WindowSize int
	// size of the window if the buffer is shrinked
	ShrinkSize int
	// maximum size of the buffer
	MaxSize int
	// target size for a block
	BlockSize int
	// minimum match len
	MinMatchLen int
	// function for computing the costs of a match or literal string if
	// offset is zero in bits. Note these costs are independent of position.
	Cost func(offset, matchLen uint32) uint32
}

// Verify checks the configuration for inconsistencies.
func (cfg *OSASConfig) Verify() error {
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
	if cfg.Cost == nil {
		return fmt.Errorf("lz: Cost must be non-nil")
	}
	return nil
}

func defaultCost(offset, matchLen uint32) uint32 {
	r := uint32(bits.Len32(matchLen))
	if offset == 0 {
		return r + 8*matchLen
	}
	return r + uint32(bits.Len32(offset))
}

// ApplyDefaults sets configuration parameters to its defaults. The code doesn't
// provide consistency.
func (cfg *OSASConfig) ApplyDefaults() {
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
	if cfg.Cost == nil {
		cfg.Cost = defaultCost
	}
}

func (cfg OSASConfig) NewWriteSequencer() (s WriteSequencer, err error) {
	return NewOptimalSuffixArraySequencer(cfg)
}

type OptimalSuffixArraySequencer struct {
	seqBuffer

	sa  []int32
	isa []int32

	// longest common prefix array lcp[io describes lcp of sa[i] and sa[i+1]
	lcp   []int32
	saPos int

	// index in isa where the block ends.
	blockEnd int

	blockSize   int
	minMatchLen int

	cost func(offset, matchLen uint32) uint32
}

// BlockSize returns the block size.
func (s *OptimalSuffixArraySequencer) BlockSize() int { return s.blockSize }

func NewOptimalSuffixArraySequencer(cfg OSASConfig) (s *OptimalSuffixArraySequencer, err error) {
	s = new(OptimalSuffixArraySequencer)
	err = s.Init(cfg)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (s *OptimalSuffixArraySequencer) Init(cfg OSASConfig) error {
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
	s.lcp = s.lcp[:0]
	s.cost = cfg.Cost
	return nil
}

func (s *OptimalSuffixArraySequencer) Reset() {
	s.seqBuffer.Reset()
	s.saPos = 0
	s.sa = s.sa[:0]
	s.isa = s.isa[:0]
}

func (s *OptimalSuffixArraySequencer) Requested() int {
	r := s.blockSize - s.buffered()
	if r <= 0 {
		return 0
	}
	if s.available() < r {
		s.Shrink()
		s.saPos = 0
		s.sa = s.sa[:0]
		s.isa = s.isa[:0]
	}
	return s.available()
}

func (s *OptimalSuffixArraySequencer) sort() {
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
	p := s.data[s.saPos:]
	suffix.Sort(p, s.sa)
	if n <= cap(s.isa) {
		s.isa = s.isa[:n]
	} else {
		s.isa = make([]int32, n)
	}
	for i, j := range s.sa {
		s.isa[j] = int32(i)
	}
	n--
	if n <= cap(s.lcp) {
		s.lcp = s.lcp[:n]
	} else {
		s.lcp = make([]int32, n)
	}
	j0 := s.sa[0]
	for i, j1 := range s.sa[1:] {
		s.lcp[i] = int32(matchLen(p[j0:], p[j1:]))
		j0 = j1
	}
}

// matches identifies the matches at postion s.saPos + i.
func (s *OptimalSuffixArraySequencer) matches(i int) []match {
	matchLen := s.blockEnd - i
	// look forward
	var matches []match
	j := int(s.isa[i])
	for j < len(s.sa)-1 {
		if int(s.lcp[j]) < matchLen {
			matchLen = int(s.lcp[j])
		}
		if matchLen < s.minMatchLen {
			break
		}
		j++
		offset := i - int(s.sa[j])
		if !(0 < offset && offset <= s.windowSize) {
			continue
		}
		matches = append(matches, match{
			matchLen: uint32(matchLen), offset: uint32(offset)})
	}
	// look backward
	matchLen = s.blockEnd - i
	j = int(s.isa[i] - 1)
	for j >= 0 {
		if int(s.lcp[j]) < matchLen {
			matchLen = int(s.lcp[j])
		}
		if matchLen < s.minMatchLen {
			break
		}
		offset := i - int(s.sa[j])
		j--
		if !(0 < offset && offset <= s.windowSize) {
			continue
		}
		matches = append(matches, match{
			matchLen: uint32(matchLen), offset: uint32(offset)})
	}
	return matches
}

func (s *OptimalSuffixArraySequencer) Sequence(blk *Block, flags int) (n int, err error) {
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
	s.blockEnd = i + n
	matchMap := make([][]match, 0, n)
	for ; i < s.blockEnd; i++ {
		matchMap = append(matchMap, s.matches(i))

	}
	n = optimalSequence(blk, s.data[s.w:s.w+n], matchMap, s.cost,
		s.minMatchLen, flags)
	s.w += n
	return n, nil
}
