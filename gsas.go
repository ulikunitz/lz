package lz

import (
	"fmt"
	"math"
	"reflect"

	"github.com/ulikunitz/lz/suffix"
)

// GSASConfig defines the configuration parameter for the greedy suffix array
// seqeuncer.
type GSASConfig struct {
	// maximal window size
	WindowSize int
	ShrinkSize int
	BlockSize  int
	// minimum match len
	MinMatchLen int
}

func (cfg *GSASConfig) windowConfig() WindowConfig {
	return WindowConfig{
		WindowSize: cfg.WindowSize,
		ShrinkSize: cfg.ShrinkSize,
		BlockSize:  cfg.BlockSize,
	}
}

// Verify checks the configuration for inconsistencies.
func (cfg *GSASConfig) Verify() error {
	wcfg := cfg.windowConfig()
	if err := wcfg.Verify(); err != nil {
		return err
	}
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
	if !(int64(cfg.WindowSize) <= int64(math.MaxInt32)) {
		// We manage positions only as uint32 values and so this limit
		// is necessary
		return fmt.Errorf(
			"lz: MaxSize=%d; must be less than MaxUint32=%d",
			cfg.WindowSize, maxUint32)
	}
	return nil
}

// ApplyDefaults sets configuration parameters to its defaults. The code doesn't
// provide consistency.
func (cfg *GSASConfig) ApplyDefaults() {
	wcfg := cfg.windowConfig()
	wcfg.ApplyDefaults()
	cfg.WindowSize = wcfg.WindowSize
	cfg.ShrinkSize = wcfg.ShrinkSize
	cfg.BlockSize = wcfg.BlockSize
	if cfg.WindowSize == 0 {
		cfg.WindowSize = 8 * 1024 * 1024
	}
	if cfg.MinMatchLen == 0 {
		cfg.MinMatchLen = 3
	}
}

// NewSequencer generates a new sequencer using the configuration parameters in
// the structure.
func (cfg GSASConfig) NewSequencer() (s Sequencer, err error) {
	return NewGreedySuffixArraySequencer(cfg)
}

// GreedySuffixArraySequencer provides a sequencer that uses a suffix array for
// the window and buffered data to create sequence. It looks for the two nearest
// entries that have the longest match.
//
// Since computing the suffix array is rather slow, it consumes a lot of CPU.
// Double Hash Sequencers are achieving almost the same compression rate with
// much less CPU consumption.
type GreedySuffixArraySequencer struct {
	Window

	// suffix array
	sa []int32
	// inverse suffix array
	isa []int32
	// bits marks the positions in the suffix array sa that have already
	// been processed
	bits bitset

	minMatchLen int
}

// WindowPtr returns the pointer to the window.
func (s *GreedySuffixArraySequencer) WindowPtr() *Window { return &s.Window }

// MemSize returns the consumed memory size by the
func (s *GreedySuffixArraySequencer) MemSize() uintptr {
	n := reflect.TypeOf(*s).Size()
	n += uintptr(cap(s.sa)) * reflect.TypeOf(int32(0)).Size()
	n += uintptr(cap(s.isa)) * reflect.TypeOf(int32(0)).Size()
	return n
}

// NewGreedySuffixArraySequencer creates a new value using the provided
// configuration. If the configuration has inconsistencies an error will be
// returned and the value of the return value s will be nil.
func NewGreedySuffixArraySequencer(cfg GSASConfig) (s *GreedySuffixArraySequencer, err error) {
	s = new(GreedySuffixArraySequencer)
	err = s.Init(cfg)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// Init initializes the sequencer. If the configuration has inconsistencies or
// invalid values the method returns an error.
func (s *GreedySuffixArraySequencer) Init(cfg GSASConfig) error {
	cfg.ApplyDefaults()
	var err error
	if err = cfg.Verify(); err != nil {
		return err
	}
	err = s.Window.Init(cfg.windowConfig())
	if err != nil {
		return err
	}
	s.minMatchLen = cfg.MinMatchLen
	s.sa = s.sa[:0]
	s.isa = s.isa[:0]
	s.bits.clear()
	return nil
}

// Reset puts the sequencer in the initial state.
func (s *GreedySuffixArraySequencer) Reset(data []byte) error {
	if err := s.Window.Reset(data); err != nil {
		return err
	}
	s.sa = s.sa[:0]
	s.isa = s.isa[:0]
	s.bits.clear()
	return nil
}

// sort computes the suffix array and its inverse 2gqqfor the window and all
// buffered data. The bits bitmap marks all sa entries that are part of the
// window.
func (s *GreedySuffixArraySequencer) sort() {
	n := len(s.data)
	if n > math.MaxInt32 {
		panic("n too large")
	}
	if n <= cap(s.sa) {
		s.sa = s.sa[:n]
	} else {
		s.sa = make([]int32, n)
	}
	suffix.Sort(s.data, s.sa)
	if n <= cap(s.isa) {
		s.isa = s.isa[:n]
	} else {
		s.isa = make([]int32, n)
	}
	for i, j := range s.sa {
		s.isa[j] = int32(i)
	}
	s.bits.clear()
	for i := 0; i < s.w; i++ {
		s.bits.insert(int(s.isa[i]))
	}
}

// Sequence computes the sequences for the next block. Data in the block will be
// overwritten. The NoTrailingLiterals flag is supported. It returns the number
// of bytes covered by the computed sequences. If the buffer is empty
// ErrEmptyBuffer will be returned.
//
// The method might compute the suffix array anew using the sort method.
func (s *GreedySuffixArraySequencer) Sequence(blk *Block, flags int) (n int, err error) {
	n = s.Buffered()
	if n > s.BlockSize {
		n = s.BlockSize
	}

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
	i := s.w
	if i+n > len(s.sa) {
		s.sort()
	}

	p := s.data[:len(s.sa)]
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
		o := i - f
		if !(0 < o && o < s.WindowSize) {
			i++
			continue
		}
		q := p[litIndex:i]
		blk.Sequences = append(blk.Sequences,
			Seq{
				MatchLen: uint32(m),
				LitLen:   uint32(len(q)),
				Offset:   uint32(o),
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

	n = i - s.w
	s.w = i
	return n, nil
}

// Shrink reduces the window length to provide more space for writing.
func (s *GreedySuffixArraySequencer) Shrink() int {
	oldWindowLen := s.w
	n := s.Window.shrink()
	if oldWindowLen == n {
		return n
	}
	s.sa = s.sa[:0]
	s.isa = s.isa[:0]
	s.bits.clear()
	return n
}
