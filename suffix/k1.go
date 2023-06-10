// SPDX-FileCopyrightText: © 2021 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

// Package suffix provides a suffix sort algorithm.
//
// It is based on the DivSufSort algorithm as described in the
// [Dismantling DivSufSort] paper.
//
// [DivSufSort]: https://arxiv.org/pdf/1710.01896.pdf
package suffix

import (
	"fmt"
	"testing"
)

type config struct {
	sizeThreshold   int
	trSizeThreshold int
	t               *testing.T
}

// Sort computes the suffix array. The slice sa must have the same length as t.
func Sort(t []byte, sa []int32) {
	config{
		sizeThreshold: 7,
	}.sort(t, sa)
}

// sort computes the suffix array using the A, B and B* types.
func (cfg config) sort(t []byte, sa []int32) {
	// Look for a simple error.
	if len(t) != len(sa) {
		panic(fmt.Errorf("len(t)=%d is different from len(sa)=%d",
			len(t), len(sa)))
	}

	// Check for simple cases and return.
	switch len(t) {
	case 0:
		return
	case 1:
		sa[0] = 0
		return
	case 2:
		if t[0] < t[1] {
			sa[0], sa[1] = 0, 1
		} else {
			sa[0], sa[1] = 1, 0
		}
		return
	}

	// Count the A, B and B* types. We are also computing the B* positions.
	var aBuckets [256]int32
	bBuckets, bStarBuckets := newBBucketsPair()

	var c0, c1 int
	i := int32(len(t) - 1)
	c0 = int(t[i])
	m := len(sa)
scanLoop:
	for {
		for {
			aBuckets[c0]++
			i--
			if i < 0 {
				break scanLoop
			}
			c0, c1 = int(t[i]), c0
			if c0 < c1 {
				break
			}
		}
		bStarBuckets.inc(c0, c1)
		m--
		sa[m] = int32(i)
		for {
			i--
			if i < 0 {
				break scanLoop
			}
			c0, c1 = int(t[i]), c0
			if c0 > c1 {
				break
			}
			bBuckets.inc(c0, c1)
		}
	}
	// compute the total number of B* suffixes
	m = len(sa) - m

	// Compute offsets. A-offsets point to the start of the buckets in the
	// final SA. B-offsets point to the end of the buckets in the final SA.
	// B*-offsets point to the start of the buckets in the B*-only suffix
	// array.
	i = 0
	j := int32(0)
	for c0 = 0; c0 < sigma; c0++ {
		t := i + aBuckets[c0]
		aBuckets[c0] = i + j
		i = t + bBuckets.at(c0, c0)
		for c1 = c0 + 1; c1 < sigma; c1++ {
			j += bStarBuckets.at(c0, c1)
			bStarBuckets.set(c0, c1, j)
			i += bBuckets.at(c0, c1)
		}
	}

	if m > 0 {
		// Sort the B* buckets into their right position at the front of
		// the sa array. The value is negated if the B* substring equals
		// the preceding value.
		var l int
		for l, j = range sa[len(sa)-m : len(sa)-1] {
			c0, c1 = int(t[j]), int(t[j+1])
			i = bStarBuckets.dec(c0, c1)
			sa[i] = int32(l)
		}
		l, j = m-1, sa[len(sa)-1]
		c0, c1 = int(t[j]), int(t[j+1])
		li := bStarBuckets.dec(c0, c1)
		sa[li] = int32(l)

		s := cfg.newSsorter(sa[:m], t, sa[len(sa)-m:])

		c0 = sigma - 2
		j = int32(m)
		for j > 0 {
			for c1 = sigma - 1; c0 < c1; c1-- {
				i = bStarBuckets.at(c0, c1)
				if j-i > 1 {
					s.ssort(int(i), int(j), li == i)
				}
				j = i
			}
			c0--
		}

		/*
			if err := verifyAfterSsort(t, sa, m, bStarBuckets); err != nil {
				panic(fmt.Errorf("verifyAfterSsort error %w", err))
			}
		*/

		// The isa table will be filled by the ranks. Segments that are
		// equal are filled with the maximum rank of that value.
		// Segments with different substrings are marked with the number
		// of substrings in the first position.
		isa := sa[m : 2*m]
		for i = int32(m - 1); i >= 0; i-- {
			if sa[i] >= 0 {
				j = i
				for {
					isa[sa[i]] = i
					i--
					if i < 0 || sa[i] < 0 {
						break
					}
				}
				sa[i+1] = i - j
				if i <= 0 {
					// i will never equals zero
					break
				}
			}
			j = i
			for {
				sa[i] = ^sa[i]
				isa[sa[i]] = j
				i--
				if sa[i] >= 0 {
					break
				}
			}
			isa[sa[i]] = j
		}

		// fmt.Printf("#1 sa[:m=%d]=%d\n", m, sa[:m])
		// fmt.Printf("#1 isa=%d\n", isa)

		// sort B* suffixes using the ranks
		cfg.trSort(sa[:m], isa)

		// fmt.Printf("#2 sa[:m=%d]=%d\n", m, sa[:m])
		// fmt.Printf("#2 isa=%d\n", isa)

		/*
			if err := verifyAfterTrsort(t, sa, m); err != nil {
				panic(err)
			}
		*/

		// create correct suffix array of B* by scanning t
		i := int32(len(t) - 1)
		c0 = int(t[i])
		j = int32(m)
	loop2:
		for {
			c1 = c0
			i--
			for {
				if i < 0 {
					break loop2
				}
				c0 = int(t[i])
				if c0 < c1 {
					break
				}
				c1 = c0
				i--
			}
			k := i
			for {
				i--
				if i < 0 {
					break
				}
				c0, c1 = int(t[i]), c0
				if c0 > c1 {
					break
				}
			}
			if k-i == 1 {
				k = -k
			}
			j--
			sa[isa[j]] = k
			if i < 0 {
				break
			}
		}

		// Copy sa values for B* suffixes to the right place and compute
		// offsets for the end of B-suffixes. The B* offsets will now
		// contain at (c0,sigma-1) the first position of the B offsets.
		bBuckets.set(sigma-1, sigma-1, int32(len(t)))
		k := int32(m)
		// Get a slice for the end offsets of the A buckets.
		aBucketEnds := bStarBuckets.a[(sigma-1)*sigma : sigma*sigma-1]
		for c0 = sigma - 2; c0 >= 0; c0-- {
			i := aBuckets[c0+1]
			for c1 = sigma - 1; c0 < c1; c1-- {
				i1 := bBuckets.at(c0, c1)
				bBuckets.set(c0, c1, i)
				i -= i1
				j = bStarBuckets.at(c0, c1)
				i -= k - j
				copy(sa[i:], sa[j:k])
				k = j
			}
			aBucketEnds[c0] = i - bBuckets.at(c0, c0)
			bBuckets.set(c0, c0, i)
		}

		// induce B suffixes
		for c1 = sigma - 2; c1 >= 0; c1-- {
			i = aBuckets[c1+1] - 1
			k := aBucketEnds[c1]
			for ; i >= k; i-- {
				j = sa[i]
				sa[i] = -j
				if j <= 0 {
					continue
				}
				j--
				c0 = int(t[j])
				if j > 0 && int(t[j-1]) > c0 {
					// preceding suffix is A
					j = -j
				}
				sa[bBuckets.dec(c0, c1)] = j
			}
		}

	}

	// induce A suffixes

	// the last suffix is an A-type
	j = int32(len(t) - 1)
	c0, c1 = int(t[j-1]), int(t[j])
	if c0 < c1 {
		// preceding suffix is type B
		j = -j
	}
	sa[aBuckets[c1]] = j
	aBuckets[c1]++

	// Now we can actually induce.
	for i, j := range sa {
		if j <= 0 {
			sa[i] = -j
			continue
		}
		j--
		c0 = int(t[j])
		if j > 0 && int(t[j-1]) < c0 {
			j = -j
		}
		sa[aBuckets[c0]] = j
		aBuckets[c0]++
	}
}

// sigma is the size of the alphabet. For sorting bytes the size is 2^8 = 256.
const sigma = 256

type bBuckets struct {
	a *[sigma * sigma]int32
}

func (b *bBuckets) index(c0, c1 int) int {
	return c0*sigma + c1
}

func (b *bBuckets) at(c0, c1 int) int32     { return b.a[b.index(c0, c1)] }
func (b *bBuckets) inc(c0, c1 int)          { b.a[b.index(c0, c1)]++ }
func (b *bBuckets) set(c0, c1 int, n int32) { b.a[b.index(c0, c1)] = n }

func (b *bBuckets) dec(c0, c1 int) int32 {
	k := b.index(c0, c1)
	b.a[k]--
	return b.a[k]
}

type bStarBuckets struct {
	a *[sigma * sigma]int32
}

func (b *bStarBuckets) index(c0, c1 int) int {
	return c1*sigma + c0
}

func (b *bStarBuckets) at(c0, c1 int) int32     { return b.a[b.index(c0, c1)] }
func (b *bStarBuckets) inc(c0, c1 int)          { b.a[b.index(c0, c1)]++ }
func (b *bStarBuckets) set(c0, c1 int, n int32) { b.a[b.index(c0, c1)] = n }

func (b *bStarBuckets) dec(c0, c1 int) int32 {
	k := b.index(c0, c1)
	b.a[k]--
	return b.a[k]
}

func newBBucketsPair() (b *bBuckets, bStar *bStarBuckets) {
	var a [sigma * sigma]int32
	b = &bBuckets{a: &a}
	bStar = &bStarBuckets{a: &a}
	return b, bStar
}

// The functions below allow verifications for certain points in the sort
// routine.
/*
func bStarPositions(t []byte) []int32 {
	var a []int32
	var c0, c1 int
	i := int32(len(t) - 1)
	c0 = int(t[i])
scanLoop:
	for {
		for {
			i--
			if i < 0 {
				break scanLoop
			}
			c0, c1 = int(t[i]), c0
			if c0 < c1 {
				break
			}
		}
		a = append(a, int32(i))
		for {
			i--
			if i < 0 {
				break scanLoop
			}
			c0, c1 = int(t[i]), c0
			if c0 > c1 {
				break
			}
		}
	}
	for i := 0; i < len(a)/2; i++ {
		a[i], a[len(a)-1-i] = a[len(a)-1-i], a[i]
	}
	return a
}

// verifyBStarPositions verifies all B* positions stored at the end of sa.
func verifyBStarPositions(t []byte, sa []int32, m int) error {
	a := bStarPositions(t)
	if len(a) != m {
		return fmt.Errorf("m is %d; want %d", m, len(a))
	}
	b := sa[len(sa)-m:]
	for i, k := range a {
		if k != b[i] {
			return fmt.Errorf("sa[len(a)-m+%d]=%d; want %d",
				i, b[i], k)
		}
	}
	return nil
}

func bStarSubstring(t []byte, sa []int32, m int, i int) []byte {
	if i == m-1 {
		return t[sa[len(sa)-1]:]
	}
	j := len(sa) - m + i
	return t[sa[j] : sa[j+1]+2]
}

// verifyAfterSsort verifies that sa has the correct entries after substring
// sorting.
func verifyAfterSsort(t []byte, sa []int32, m int, bStarBuckets *bStarBuckets) error {
	var err error
	if err = verifyBStarPositions(t, sa, m); err != nil {
		return err
	}
	for i := 0; i < m-1; i++ {
		a, b := absNeg(sa[i]), absNeg(sa[i+1])
		u := bStarSubstring(t, sa, m, int(a))
		v := bStarSubstring(t, sa, m, int(b))
		switch bytes.Compare(u, v) {
		case 0:
			if sa[i+1] >= 0 {
				return fmt.Errorf("sa[%d]=%d >= 0; want < 0",
					i+1, sa[i+1])
			}
		case 1:
			return fmt.Errorf("substring for sa[%d]=%d > sa[%d]=%d",
				i, sa[i], i+1, sa[i+1])
		}

	}
	return nil
}

func verifyAfterTrsort(t []byte, sa []int32, m int) error {
	a := bStarPositions(t)
	if m != len(a) {
		return fmt.Errorf("m=%d; want %d", m, len(a))
	}
	isa := sa[m : 2*m]
	b := make([]int32, m)
	for i, r := range isa {
		b[r] = int32(i)
	}

	for i := 0; i < len(b)-1; i++ {
		u := t[a[b[i]]:]
		v := t[a[b[i+1]]:]
		if bytes.Compare(u, v) >= 0 {
			return fmt.Errorf("i=%d t[a[%d]=%d:] >= t[a[%d]=%d:],"+
				" b=%d, isa=%d",
				i, b[i], a[b[i]], b[i+1], a[b[i+1]], b, isa)
		}
	}

	return nil
}
*/
