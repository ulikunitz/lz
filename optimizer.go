package lz

import (
	"fmt"
	"math"
)

// We have to rethink this again.
//
// We keep it simply we maintain a descending list of matches (m, o) for each
// position. So m_i > m_j and o_i > o_j with i < j. Literals are implicit.

type match struct {
	m, o uint32
}



type matchMap [][]match

type optrec struct {
	c        uint32
	off      uint32
	matchLen uint32
}

type optimizer struct {
	blk         *Block
	p           []byte
	m           matchMap
	cost        costFn
	minMatchLen uint32
	flags       int
	a           []optrec
}

type costFn func(offset, matchLen uint32) uint32

func (o *optimizer) sequence() int {
	n := len(o.p)
	if len(o.m) != n {
		panic(fmt.Errorf("len(o.m)=%d != len(o.p)=%d",
			len(o.m), len(o.p)))
	}

	if n+1 < cap(o.a) {
		o.a = o.a[:n+1]
	} else {
		o.a = make([]optrec, n+1)
	}
	for i := range o.a {
		o.a[i] = optrec{c: math.MaxUint32}
	}

	// TODO: optimize, extend current match and stop if current match
	// doesn't update costs; don't do the initial stuff
	for i, m := range o.m {
		ml := uint32(n + 1 - i)
		off := uint32(0)
		r := o.a[i]
		for _, x := range m {
			for ; ml > x.m; ml-- {
				c := r.c + o.cost(off, ml)
				p := &o.a[i+int(ml)]
				if c < p.c {
					*p = optrec{c: c, matchLen: ml,
						off: off}
				}
			}
			off = x.o
		}
		for ; ml >= o.minMatchLen; ml-- {
			c := r.c + o.cost(off, ml)
			p := &o.a[i+int(ml)]
			if c < p.c {
				*p = optrec{c: c, matchLen: ml, off: off}
			}
		}
		off = 0
		for ; ml > 0; ml-- {
			c := r.c + o.cost(off, ml)
			p := &o.a[i+int(ml)]
			if c < p.c {
				*p = optrec{c: c, matchLen: ml, off: off}
			}
		}
	}

	m := o.m[0][:0]

	// reconstruct seq and handle literals
	i := n
	for i > 0 {
		r := o.a[i]
		m = append(m, match{m: r.matchLen, o: r.off})
		i -= int(r.matchLen)
	}
	if i < 0 {
		panic("matchLen issue")
	}

	sequences := o.blk.Sequences[:0]
	literals := o.blk.Literals[:0]
	var seq Seq
	i = 0
	for j := len(m) - 1; j >= 0; j-- {
		u := m[j]
		if u.o == 0 {
			seq.LitLen += u.m
			k := i + int(u.m)
			literals = append(literals, o.p[i:k]...)
			i = k
		} else {
			seq.Offset = u.o
			seq.MatchLen = u.m
			i += int(u.m)
			sequences = append(sequences, seq)
			seq = Seq{}
		}
	}

	if seq.LitLen > 0 {
		if o.flags&NoTrailingLiterals != 0 {
			literals = literals[:len(literals)-int(seq.LitLen)]
		} else {
			sequences = append(sequences, seq)
		}
	}

	o.blk.Sequences = sequences
	o.blk.Literals = literals

	return n
}
