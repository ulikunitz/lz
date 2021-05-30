package lz

import (
	"fmt"
)

type match struct {
	offset   uint32
	matchLen uint32
}

func optimalSequence(
	blk *Block,
	p []byte,
	matchMap [][]match,
	cost func(offset, matchLen uint32) uint32,
	minMatchLen int,
	flags int) int {

	n := len(matchMap)
	if n != len(p) {
		panic(fmt.Errorf("len(p)=%d != len(matchMap)=%d", len(p), n))
	}

	fmt.Printf("n: %d\n", n)

	// d stores minimal costs for single steps from position i to position
	// j. The transpose stores the offset for the step.
	var d matrix
	d.init(n + 1)

	// fill d with costs for literal steps. Offsets are already zero.
	for i := 0; i < n; i++ {
		for j := i + 1; j <= n; j++ {
			d.set(i, j, cost(0, uint32(j-i)))
		}
	}

	// Now apply all matches to d.
	for k, matches := range matchMap {
		for _, m := range matches {
			if m.offset == 0 || m.matchLen < uint32(minMatchLen) {
				panic(fmt.Errorf("match %+v invalid", m))
			}
			l := k + int(m.matchLen)
			if l > n {
				panic(fmt.Errorf("match %+v too long", m))
			}
			for i := k; i <= l-minMatchLen; i++ {
				off := m.offset + uint32(i-k)
				for j := l; j >= i+minMatchLen; j-- {
					c := cost(off, uint32(j-i))
					if c >= d.at(i, j) {
						// We assume that the shorter
						// matches will not provide
						// improvement.
						break
					}
					d.set(i, j, c)
					d.setT(i, j, off)
				}
			}
		}
	}

	// Find optimal sequence of steps generating the minumum costs.
	costs := make([]uint32, n+1)
	step := make([]uint32, n+1)
	for j := 1; j <= n; j++ {
		// cost for one step
		k, c := 0, d.at(0, j)
		// multiple steps better
		for i := 1; i < j; i++ {
			if t := costs[i] + d.at(i, j); t < c {
				k, c = i, t
			}
		}
		costs[j] = c
		step[j] = uint32(k)
	}

	// retracking steps
	i, j := int(step[n]), n
	for j > 0 {
		i, j, step[i] = int(step[i]), i, uint32(j)
	}

	// reconstruct sequences
	var seq Seq
	for i = 0; i < n; i = j {
		j = int(step[i])
		offset, matchLen := d.atT(i, j), uint32(j-i)
		if offset == 0 {
			seq.LitLen += matchLen
			blk.Literals = append(blk.Literals, p[i:j]...)
			continue
		}
		seq.Offset = offset
		seq.MatchLen = matchLen
		blk.Sequences = append(blk.Sequences, seq)
		seq = Seq{}
	}

	if seq.LitLen > 0 && flags&NoTrailingLiterals != 0 {
		l := int(seq.LitLen)
		blk.Literals = blk.Literals[:len(blk.Literals)-l]
		return n - l
	}

	return n
}

// matrix stores a square matrix of uint32 values.
type matrix struct {
	v []uint32
	n int
}

// init initializes the matrix.
func (m *matrix) init(n int) {
	m.v = make([]uint32, n*n)
	m.n = n
}

// at returns the value in cell i,j.
func (m *matrix) at(i, j int) uint32 {
	return m.v[i*m.n+j]
}

// atT returns the value in cell j,i. Think about this as the access to the
// transposed matrix.
func (m *matrix) atT(i, j int) uint32 {
	return m.v[j*m.n+i]
}

// set puts x into the cell i,j
func (m *matrix) set(i, j int, x uint32) {
	m.v[i*m.n+j] = x
}

// setT puts x into the cell j,i. This is like the access to the transposed
// matrix.
func (m *matrix) setT(i, j int, x uint32) {
	m.v[j*m.n+i] = x
}
