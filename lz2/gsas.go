package lz2

import (
	"fmt"
	"math"

	"github.com/ulikunitz/lz/suffix"
)

// GSASConfig defines the configuration parameter for the greedy suffix array
// sequencer.
type GSASConfig struct {
	ShrinkSize int
	BufferSize int
	WindowSize int
	BlockSize  int

	// minimum match len
	MinMatchLen int
}

// Verify checks the configuration for inconsistencies.
func (cfg *GSASConfig) Verify() error {
	bc := BufferConfig(cfg)
	if err := bc.Verify(); err != nil {
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
	bc := BufferConfig(cfg)
	bc.ApplyDefaults()
	SetBufferConfig(cfg, bc)
	if cfg.MinMatchLen == 0 {
		cfg.MinMatchLen = 3
	}
}

// NewSequencer generates a new sequencer using the configuration parameters in
// the structure.
func (cfg GSASConfig) NewSequencer() (s Sequencer, err error) {
	gsas := new(greedySuffixArraySequencer)
	err = gsas.init(cfg)
	if err != nil {
		return nil, err
	}
	return gsas, nil
}

// greedySuffixArraySequencer provides a sequencer that uses a suffix array for
// the window and buffered data to create sequence. It looks for the two nearest
// entries that have the longest match.
//
// Since computing the suffix array is rather slow, it consumes a lot of CPU.
// Double Hash Sequencers are achieving almost the same compression rate with
// much less CPU consumption.
type greedySuffixArraySequencer struct {
	data []byte

	// suffix array
	sa []int32
	// inverse suffix array
	isa []int32
	// bits marks the positions in the suffix array sa that have already
	// been processed
	bits bitset

	w int

	GSASConfig
}

// init initializes the sequencer. If the configuration has inconsistencies or
// invalid values the method returns an error.
func (s *greedySuffixArraySequencer) init(cfg GSASConfig) error {
	cfg.ApplyDefaults()
	var err error
	if err = cfg.Verify(); err != nil {
		return err
	}

	s.data = s.data[:0]
	s.sa = s.sa[:0]
	s.isa = s.isa[:0]
	s.bits.clear()
	s.GSASConfig = cfg
	return nil
}

func (s *greedySuffixArraySequencer) Update(data []byte, delta int) {
	switch {
	case delta > 0:
		s.w = doz(s.w, delta)
	case delta < 0 || s.w > len(data):
		s.w = 0
	}
	s.sa = s.sa[:0]
	s.isa = s.isa[:0]
	s.bits.clear()

	if len(data) == 0 {
		s.data = s.data[:0]
		return
	}
	s.data = data
}

func (s *greedySuffixArraySequencer) Config() SeqConfig {
	return &s.GSASConfig
}

// sort computes the suffix array and its inverse for the window and all
// buffered data. The bits bitmap marks all sa entries that are part of the
// window.
func (s *greedySuffixArraySequencer) sort() {
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
func (s *greedySuffixArraySequencer) Sequence(blk *Block, flags int) (n int, err error) {
	n = len(s.data) - s.w
	if n > s.BlockSize {
		n = s.BlockSize
	}

	if blk == nil {
		if n == 0 {
			return 0, ErrNoData
		}
		s.w += n
		return n, nil
	}
	blk.Sequences = blk.Sequences[:0]
	blk.Literals = blk.Literals[:0]
	if n == 0 {
		return 0, ErrNoData
	}
	i := s.w
	if i+n > len(s.sa) {
		s.sort()
	}

	p := s.data[:i+n]
	litIndex := i
	for ; i < len(p); i++ {
		j := int(s.isa[i])
		s.bits.insert(j)
		k1, ok1 := s.bits.memberBefore(j)
		k2, ok2 := s.bits.memberAfter(j)
		var f, m int
		if ok1 {
			f = int(s.sa[k1])
			m = lcp(p[f:], p[i:])
		}
		if ok2 {
			f2 := int(s.sa[k2])
			m2 := lcp(p[f2:], p[i:])
			if m2 > m || (m2 == m && f2 > f) {
				f, m = f2, m2
			}
		}
		if m < s.MinMatchLen {
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

	if flags&NoTrailingLiterals != 0 && len(blk.Sequences) > 0 {
		i = litIndex
	} else {
		blk.Literals = append(blk.Literals, p[litIndex:]...)
		i = len(p)
	}

	n = i - s.w
	s.w = i
	return n, nil
}
