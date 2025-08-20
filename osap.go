// SPDX-FileCopyrightText: © 2021 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

package lz

import (
	"fmt"
	"math"
	"math/bits"
	"sort"
	"strings"

	"github.com/ulikunitz/lz/suffix"
	"golang.org/x/exp/slices"
)

// XZCost models the cost of the bits going into the XZ encoding. The maximum edge
// length is 273.
func XZCost(m, o uint32) uint64 {
	if o == 0 {
		return 9 * uint64(m)
	}

	c := uint64(0)
	m -= 2
	switch {
	case m < 8:
		c += 4
	case m < 16:
		c += 5
	default:
		c += 10
	}
	if d := o - 1; d < 4 {
		c += 4
	} else {
		c += 2 + uint64(bits.Len32(d))
	}
	return c
}

// OSAPConfig provides the configuration parameters for the Optimizing Suffix
// Array Parser (OSAP).
type OSAPConfig struct {
	ShrinkSize int
	BufferSize int
	WindowSize int
	BlockSize  int

	MinMatchLen int
	MaxMatchLen int

	Cost string
}

// Clone creates a copy of the configuration.
func (cfg *OSAPConfig) Clone() ParserConfig {
	x := *cfg
	return &x
}

// UnmarshalJSON parses the JSON value and sets the fields of OSAPConfig.
func (cfg *OSAPConfig) UnmarshalJSON(p []byte) error {
	*cfg = OSAPConfig{}
	return unmarshalJSON(cfg, "OSAP", p)
}

// MarshalJSON creates the JSON string for the configuration. Note that it adds
// a property Type with value "OSAP" to the structure.
func (cfg *OSAPConfig) MarshalJSON() (p []byte, err error) {
	return marshalJSON(cfg, "OSAP")
}

// BufConfig returns the [BufConfig] value for the OSAP configuration.
func (cfg *OSAPConfig) BufConfig() BufConfig {
	return bufferConfig(cfg)
}

// SetBufConfig sets the buffer configuration parameters of the parser
// configuration.
func (cfg *OSAPConfig) SetBufConfig(bc BufConfig) {
	setBufferConfig(cfg, bc)
}

// SetDefaults sets the defaults for the zero values of the the OSAP
// configuration.
func (cfg *OSAPConfig) SetDefaults() {
	bc := bufferConfig(cfg)
	if bc.BufferSize == 0 {
		bc.SetDefaults()
		bc.BufferSize = bc.WindowSize
	} else {
		bc.SetDefaults()
	}
	setBufferConfig(cfg, bc)

	if cfg.MinMatchLen == 0 {
		cfg.MinMatchLen = 3
	}
	if cfg.MaxMatchLen == 0 {
		cfg.MaxMatchLen = 273
	}

	if cfg.Cost == "" {
		cfg.Cost = "XZCost"
	}
}

// Verify verifies the configuration for the Optimizing Suffix Array Parser.
func (cfg *OSAPConfig) Verify() error {
	var err error
	bc := bufferConfig(cfg)
	if err = bc.Verify(); err != nil {
		return err
	}

	if !(2 <= cfg.MinMatchLen && cfg.MinMatchLen <= cfg.MaxMatchLen) {
		return fmt.Errorf("lz: MinMatchLen=%d must be in range [%d..MaxMatchLen=%d",
			cfg.MinMatchLen, 2, cfg.MaxMatchLen)
	}

	switch cfg.Cost {
	case "XZCost":
		break
	default:
		return fmt.Errorf("lz.OSAPConfig: Cost string must not be empty")
	}

	return nil
}

// NewParser returns the Optimizing Parser Array Parser.
func (cfg *OSAPConfig) NewParser() (s Parser, err error) {
	osas := new(optSuffixArrayParser)
	if err = osas.init(*cfg); err != nil {
		return nil, err
	}
	return osas, nil
}

type edge struct {
	m uint32
	o uint32
}

type optSuffixArrayParser struct {
	Buffer

	edgeBuf []edge
	edges   [][]edge
	start   int
	nEdges  int

	tmp []edge

	cost func(m, o uint32) uint64

	OSAPConfig
}

func (s *optSuffixArrayParser) ParserConfig() ParserConfig {
	return &s.OSAPConfig
}

func (s *optSuffixArrayParser) init(cfg OSAPConfig) error {
	cfg.SetDefaults()
	var err error
	if err = cfg.Verify(); err != nil {
		return err
	}
	bc := bufferConfig(&cfg)
	if err = s.Buffer.Init(bc); err != nil {
		return err
	}

	s.resetEdges()

	switch cfg.Cost {
	case "XZCost":
		s.cost = XZCost
	}

	s.OSAPConfig = cfg
	return nil
}

func (s *optSuffixArrayParser) Reset(data []byte) error {
	err := s.Buffer.Reset(data)
	if err != nil {
		return err
	}

	s.resetEdges()
	return nil
}

func (s *optSuffixArrayParser) Shrink() int {
	delta := s.Buffer.Shrink()
	if delta > 0 {
		s.resetEdges()
	}
	return delta
}

/* TODO: remove
func reverse[T any](s []T) {
	i, j := 0, len(s)-1
	for i < j {
		s[i], s[j] = s[j], s[i]
		i++
		j--
	}
}
*/

const edgeStats = false

func computeEdgeStats(edges [][]edge) string {
	lengths := make([]int, len(edges))
	for i, e := range edges {
		lengths[i] = len(e)
	}
	sort.Slice(lengths, func(i, j int) bool {
		return lengths[i] < lengths[j]
	})
	var sb strings.Builder
	for i, p := range []int{0, 25, 50, 75, 90, 95, 99, 100} {
		if i > 0 {
			fmt.Fprint(&sb, ", ")
		}
		k := p * len(lengths) / 100
		if k >= len(lengths) {
			k = len(lengths) - 1
		}
		fmt.Fprintf(&sb, "%2d%% %d", p, lengths[k])
	}
	return sb.String()
}

func (s *optSuffixArrayParser) resetEdges() {
	s.edgeBuf = s.edgeBuf[:0]
	s.edges = s.edges[:0]
	s.start = 0
	s.nEdges = 0
	s.tmp = s.tmp[:0]
}

func (s *optSuffixArrayParser) computeEdges() {
	data := s.Data
	if len(data) > math.MaxInt32 {
		panic(fmt.Errorf("lz: len(data)=%d too large", len(data)))
	}

	// Right size edges slice of slice and clean it.
	s.start = s.W
	k := len(data) - s.start
	if k < cap(s.edges) {
		s.edges = s.edges[:k]
	} else {
		s.edges = make([][]edge, k)
	}
	k *= 4
	if k < cap(s.edgeBuf) {
		s.edgeBuf = s.edgeBuf[:k]
	} else {
		s.edgeBuf = make([]edge, k)
	}

	// We need to make the access to the edges slices cache friendly.
	// Statistics showed that 95% the edges entry will not have more than 4
	// entries.
	for i := range s.edges {
		k := i * 4
		s.edges[i] = s.edgeBuf[k : k : k+4]
	}
	s.nEdges = 0

	if len(data) == 0 {
		return
	}

	winStart := doz(s.W, s.WindowSize)

	// Compute suffix array sa, inverse suffix array sainv and the lcp
	// table.
	t := data[winStart:]
	sa := make([]int32, len(t))
	suffix.Sort(t, sa)
	lcp := make([]int32, len(sa))
	suffix.LCP(t, sa, nil, lcp)

	// Check for maximum length in the table.
	maxLen := int32(0)
	for _, n := range lcp {
		if n > maxLen {
			maxLen = n
		}
	}
	if int(maxLen) > s.MaxMatchLen {
		maxLen = int32(s.MaxMatchLen)
	}

	// index offset to convert suffix indexes into edges indexes
	w := int32(winStart - s.start)

	// f is called for each segment of common prefixes. We sort the segment
	// and fill the edges entries using the predecessors. Note we never
	// have to compute the edge length or access the original text.
	f := func(m int, seg []int32) {
		/*
			slices.SortStableFunc(seg, func(x, y int32) bool {
				return x < y
			})
		*/
		/*
			slices.SortFunc(seg, func(x, y int32) bool {
				return x < y
			})
		*/
		slices.Sort(seg)
		for j := len(seg) - 1; j > 0; j-- {
			i := seg[j]
			// k is the index into the edges slice. If it is too
			// small we can stop.
			k := i + w
			if k < 0 {
				break
			}
			o := uint32(i - seg[j-1])
			if o > uint32(s.WindowSize) {
				continue
			}
			p := &s.edges[k]
			if len(*p) > 0 {
				if (*p)[len(*p)-1].o <= o {
					continue
				}
			}
			s.nEdges++
			*p = append(*p, edge{m: uint32(m), o: o})
		}
	}
	suffix.Segments(sa, lcp, s.MinMatchLen, int(maxLen), f)

	if edgeStats {
		fmt.Println(computeEdgeStats(s.edges))
	}

	/*
		// save memory and make access to the edges array more cache friendly.
		tmp := make([]edge, s.nEdges)
		j := 0
		for i, e := range s.edges {
			k := j + len(e)
			s.edges[i] = tmp[j:k:k]
			j = k
			copy(s.edges[i], e)
		}
	*/
}

// shortestPath appends the shortest path in reversed order
func (s *optSuffixArrayParser) shortestPath(p []edge, n int) []edge {
	k := s.W - s.start
	edges := s.edges[k : k+n]

	type opt struct {
		m, o uint32
		c    uint64
	}

	d := make([]opt, n+1)
	for i := range d {
		if i == 0 {
			continue
		}
		d[i] = opt{m: 1, o: 0, c: s.cost(uint32(i), 0)}
	}

	for i, q := range edges {
		ci := d[i].c
		maxLen := uint32(n - i)
		for k := len(q) - 1; k >= 0; k-- {
			max := q[k].m
			if max > maxLen {
				max = maxLen
			}
			o := q[k].o
			for m := uint32(s.MinMatchLen); m <= max; m++ {
				c := ci + s.cost(m, o)
				j := i + int(m)
				if c < d[j].c {
					d[j] = opt{m: m, o: o, c: c}
				}
			}
		}
	}

	i := uint32(n)
	for i != 0 {
		m, o := d[i].m, d[i].o
		p = append(p, edge{m: m, o: o})
		i -= m
	}
	return p
}

func (s *optSuffixArrayParser) Parse(blk *Block, flags int) (n int, err error) {
	n = len(s.Data) - s.W
	if n > s.BlockSize {
		n = s.BlockSize
	}

	if blk == nil {
		if n == 0 {
			return 0, ErrEmptyBuffer
		}
		return n, nil
	}

	blk.Sequences = blk.Sequences[:0]
	blk.Literals = blk.Literals[:0]

	if n == 0 {
		return 0, ErrEmptyBuffer
	}

	if s.W+n > s.start+len(s.edges) {
		s.computeEdges()
	}

	if s.nEdges == 0 {
		w := s.W
		s.W += n
		blk.Literals = append(blk.Literals, s.Data[w:s.W]...)
		return n, nil
	}

	sp := s.shortestPath(s.tmp[:0], n)
	i := uint32(s.W)
	litIndex := i
	p := s.Data[:s.W+n]
	for j := len(sp) - 1; j >= 0; j-- {
		e := sp[j]
		if e.o == 0 {
			i += e.m
			continue
		}
		q := p[litIndex:i]
		blk.Sequences = append(blk.Sequences,
			Seq{
				LitLen:   uint32(len(q)),
				MatchLen: e.m,
				Offset:   e.o,
			})
		blk.Literals = append(blk.Literals, q...)
		i += e.m
		litIndex = i
	}
	if flags&NoTrailingLiterals != 0 && len(blk.Sequences) > 0 {
		i = litIndex
	} else {
		blk.Literals = append(blk.Literals, p[litIndex:]...)
		i = uint32(len(p))
	}
	n = int(i) - s.W
	s.W = int(i)
	return n, nil
}
