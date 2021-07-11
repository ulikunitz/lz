package lz

import (
	"sort"
)

// We need to rethink this.
// So we want to have a list of matches represented as tuples (a, b, o) where a
// is the index in the block of the start of the match and b the first index
// after the match and o the offset. An offset of 0 could identify a literal
// copy but the sequencers don't need to provide those to the optimizer.
//
// In a first step the optimizer would sort the matches after the a component
// followed by the offset and then the b component.
//
// In the second step all shadowed matched can be removed. A match is shadowed
// if there is a match that covers the match but has a smaller offset. We are
// doing it by scanning through the matches and keep two sets: a life set and
// copy survivors to the front of the slice.
//
// Now the idea is the old one: Start with position zero and map out all the
// ways to reach other positions. Do it for the second position but only update
// if you there is a smaller cost. To do it we are keeping a set of life
// matches. Literals need only be regarded for the next positions, we can always
// extned them. We need to store the cost, the last postiion and the offset for
// position in the block.

// match is for a match in inside the block. The fields a and b are indexes into
// the block that mark the start and end of a match with offset o. It can also
// be used to mark a literals chain by using the offset 0.
type match struct {
	a, b, o uint32
}

func (m match) Len() uint32 {
	return m.b - m.a
}

type matchSorter struct {
	a    []match
	less func(x, y match) bool
}

func (s *matchSorter) Len() int { return len(s.a) }

func (s *matchSorter) Swap(i, j int) {
	s.a[i], s.a[j] = s.a[j], s.a[i]
}

func (s *matchSorter) Less(i, j int) bool {
	return s.less(s.a[i], s.a[j])
}

func reduceMatches(matches []match, n int) []match {
	// sort for o up a up b down
	sort.Sort(&matchSorter{matches, func(x, y match) bool {
		switch {
		case x.o < y.o:
			return true
		case x.o > y.o:
			return false
		case x.a < y.a:
			return true
		case x.a > y.a:
			return false
		default:
			return x.b > y.b
		}
	}})

	// merge all overlapping or contacting matches and ignore shadows
	var c match
	w := 0
	// shadow array
	s := make([]uint32, n)
	for _, x := range matches {
		// ignore everything that doesn't make sense
		if !(x.o > 0 && x.a < x.b && int64(x.a) < int64(n)) {
			continue
		}
		if int64(x.b) >= int64(n) {
			x.b = uint32(n)
		}
		// ignore shadowed matches
		if s[x.a] >= x.b {
			continue
		}
		// new offset?
		if c.o < x.o {
			// former current match?
			if c.o > 0 {
				// output c
				matches[w] = c
				w++
				// throw shade
				for i := c.a; i < c.b; i++ {
					s[i] = c.b
				}
				// shadow?
				if c.a <= x.a && x.b <= c.b {
					// reset c
					x = match{}
				}
			}
			c = x
			continue
		}
		// overlapping or contacting
		if c.b >= x.a {
			if x.b > c.b {
				c.b = x.b
			}
			continue
		}
		// there is a distance between c and x
		// output c
		matches[w] = c
		w++
		// throw shade
		for i := c.a; i < c.b; i++ {
			s[i] = c.b
		}
		c = x
	}
	if c.o > 0 {
		matches[w] = c
		w++
	}
	matches = matches[:w]

	// stable sort results in a up o up b down
	sort.Stable(&matchSorter{a: matches, less: func(x, y match) bool {
		return x.a < y.a
	}})

	return matches
}

type optrec struct {
	c        uint32
	off      uint32
	matchLen uint32
}

type optimizer struct {
	blk         *Block
	p           []byte
	m           []match
	cost        costFn
	minMatchLen uint32
	flags       int
	a           []optrec
}

type costFn func(offset, matchLen uint32) uint32

func (o *optimizer) addMatch(i int, matchLen uint32, offset uint32) {
	a := o.a[i:]
	or := a[0]
	max := matchLen
	if int64(len(a)) <= int64(max) {
		if len(a) == 0 {
			panic("array a too small")
		}
		max = uint32(len(a)) - 1
	}
	var c, mlen uint32
	for n := max; n >= o.minMatchLen; n-- {
		if or.off == offset && or.matchLen > 0 {
			mlen = or.matchLen + n
			c = o.a[i-int(or.matchLen)].c
		} else {
			mlen = n
			c = or.c
		}
		c += o.cost(offset, mlen)
		p := &a[n]
		if c >= p.c && p.c > 0 {
			break
		}
		p.c = c
		p.matchLen = mlen
		p.off = offset
	}
}

func (o *optimizer) sequence() int {
	n := len(o.p)
	o.m = reduceMatches(o.m, n)
	// Now we need to manage a life set and a slice of struc
	var l bitset
	// index into m
	k := 0
	if n+1 <= cap(o.a) {
		o.a = o.a[:n+1]
		for i := range o.a {
			o.a[i] = optrec{}
		}
	} else {
		o.a = make([]optrec, n+1)
	}
	mml := o.minMatchLen
	for i := 0; i < n; i++ {
		// add all live matches and remove those that will become dead
		for j, ok := l.firstMember(); ok; j, ok = l.memberAfter(j) {
			u := o.m[j]
			t := u.b - uint32(i)
			o.addMatch(i, t, u.o)
			if t <= o.minMatchLen {
				l.delete(j)
			}
		}
		// add all new matches
		for k < len(o.m) {
			u := o.m[k]
			if int64(i) < int64(u.a) {
				break
			}
			t := int64(u.b) - int64(i)
			if t >= int64(o.minMatchLen) {
				o.addMatch(i, uint32(t), u.o)
				if t > int64(o.minMatchLen) {
					l.insert(k)
				}
			}
			k++
		}
		// add one literal to move forward
		o.minMatchLen = 1
		o.addMatch(i, 1, 0)
		o.minMatchLen = mml
	}

	// reconstruct seq and handle literals
	o.m = o.m[:0]
	i := n
	for i > 0 {
		r := o.a[i]
		o.m = append(o.m, match{b: r.matchLen, o: r.off})
		i -= int(r.matchLen)
	}
	if i < 0 {
		panic("matchLen issue")
	}

	sequences := o.blk.Sequences[:0]
	literals := o.blk.Literals[:0]
	var seq Seq
	i = 0
	for j := len(o.m) - 1; j >= 0; j-- {
		u := o.m[j]
		if u.o == 0 {
			seq.LitLen += u.b
			k = i + int(u.b)
			literals = append(literals, o.p[i:k]...)
			i = k
		} else {
			seq.Offset = u.o
			seq.MatchLen = u.b
			i += int(u.b)
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
