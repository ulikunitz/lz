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
		if !(x.o > 0 && x.a < x.b && int(x.b) <= n) {
			continue
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

func optimalSequence(
	blk *Block,
	p []byte,
	m []match,
	cost func(offset, matchLen uint32) uint32,
	minMatchLen int,
	flags int) int {

	n := len(p)

	m = reduceMatches(m, n)

	_ = m
	return n
}
