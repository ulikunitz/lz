// SPDX-FileCopyrightText: © 2021 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

package lz2

import (
	"fmt"
	"math"
	"math/bits"
	"sort"
	"strings"

	"github.com/ulikunitz/lz/suffix"
	"golang.org/x/exp/slices"
)

// Models the cost of the bits going into the XZ encoding. The maximum edge
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

type OSASConfig struct {
	ShrinkSize int
	BufferSize int
	WindowSize int
	BlockSize  int

	MinMatchLen int
	MaxMatchLen int

	Cost func(m, o uint32) uint64
}

func (cfg *OSASConfig) ApplyDefaults() {
	bc := BufferConfig(cfg)
	if bc.BufferSize == 0 {
		bc.ApplyDefaults()
		bc.BufferSize = bc.WindowSize
	} else {
		bc.ApplyDefaults()
	}
	SetBufferConfig(cfg, bc)

	if cfg.MinMatchLen == 0 {
		cfg.MinMatchLen = 3
	}
	if cfg.MaxMatchLen == 0 {
		cfg.MaxMatchLen = 273
	}

	if cfg.Cost == nil {
		cfg.Cost = XZCost
	}
}

func (cfg *OSASConfig) Verify() error {
	var err error
	bc := BufferConfig(cfg)
	if err = bc.Verify(); err != nil {
		return err
	}

	if !(2 <= cfg.MinMatchLen && cfg.MinMatchLen <= cfg.MaxMatchLen) {
		return fmt.Errorf("lz: MinMatchLen=%d must be in range [%d..MaxMatchLen=%d",
			cfg.MinMatchLen, 2, cfg.MaxMatchLen)
	}

	if cfg.Cost == nil {
		return fmt.Errorf("lz: Cost function must not be nil")
	}

	return nil
}

func (cfg *OSASConfig) NewSequencer() (s Sequencer, err error) {
	osas := new(optSuffixArraySequencer)
	if err = osas.init(*cfg); err != nil {
		return nil, err
	}
	return osas, nil
}

type edge struct {
	m uint32
	o uint32
}

type optSuffixArraySequencer struct {
	data []byte
	w    int

	edgeBuf []edge
	edges   [][]edge
	start   int
	nEdges  int

	tmp []edge

	OSASConfig
}

func (s *optSuffixArraySequencer) init(cfg OSASConfig) error {
	cfg.ApplyDefaults()
	if err := cfg.Verify(); err != nil {
		return err
	}
	s.OSASConfig = cfg
	return nil
}

func (s *optSuffixArraySequencer) Config() SeqConfig {
	return &s.OSASConfig
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

func (s *optSuffixArraySequencer) computeEdges(data []byte) {
	if len(data) > math.MaxInt32 {
		panic(fmt.Errorf("lz: len(data)=%d too large", len(data)))
	}

	s.data = data

	// Right size edges slice of slice and clean it.
	s.start = s.w
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

	winStart := doz(s.w, s.WindowSize)

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

func (s *optSuffixArraySequencer) Update(data []byte, delta int) {
	switch {
	case delta > 0:
		s.w = doz(s.w, delta)
	case delta < 0 || s.w > len(data):
		s.w = 0
	}

	s.computeEdges(data)
}

// shortestPath appends the shortest path in reversed order
func (s *optSuffixArraySequencer) shortestPath(p []edge, n int) []edge {
	k := s.w - s.start
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
		d[i] = opt{m: 1, o: 0, c: s.Cost(uint32(i), 0)}
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
				c := ci + s.Cost(m, o)
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

func (s *optSuffixArraySequencer) Sequence(blk *Block, flags int) (n int, err error) {
	n = len(s.data) - s.w
	if n > s.BlockSize {
		n = s.BlockSize
	}

	if blk == nil {
		if n == 0 {
			return 0, ErrNoData
		}
		return n, nil
	}

	blk.Sequences = blk.Sequences[:0]
	blk.Literals = blk.Literals[:0]

	if n == 0 {
		return 0, ErrNoData
	}
	if s.nEdges == 0 {
		w := s.w
		s.w += n
		blk.Literals = append(blk.Literals, s.data[w:s.w]...)
		return n, nil
	}

	sp := s.shortestPath(s.tmp[:0], n)
	i := uint32(s.w)
	litIndex := i
	p := s.data[:s.w+n]
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
	n = int(i) - s.w
	s.w = int(i)
	return n, nil
}
