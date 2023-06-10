// SPDX-FileCopyrightText: © 2021 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

package suffix

import "fmt"

func (cfg config) trSort(sa []int32, isa []int32) {
	if cfg.trSizeThreshold == 0 {
		cfg.trSizeThreshold = 8
	}

	budget := budget{
		chance: ilog2(len(sa)) * 2 / 3,
		remain: len(sa),
		incval: len(sa),
	}

	for depth := 1; -sa[0] < int32(len(sa)); depth *= 2 {
		f := 0
		skip := 0
		unsorted := 0
		for {
			t := int(sa[f])
			if t < 0 {
				f -= t
				skip += t
			} else {
				if skip != 0 {
					sa[f+skip] = int32(skip)
					skip = 0
				}
				b := int(isa[t] + 1)
				if b-f > 1 {
					budget.count = 0
					cfg.trIntroSort(sa, isa, depth, f, b,
						&budget)
					if budget.count != 0 {
						unsorted += budget.count
					} else {
						skip = f - b
					}
				} else if b-f == 1 {
					skip = -1
				}
				f = b
			}
			if f >= len(sa) {
				break
			}
		}
		if skip != 0 {
			sa[f+skip] = int32(skip)
		}
		if unsorted == 0 {
			break
		}
	}
}

type stackEntry struct{ a, b, c, d, e int }

type stack []stackEntry

func (s *stack) push(a, b, c, d, e int) {
	*s = append(*s, stackEntry{a, b, c, d, e})
}

func (s *stack) pop() (a, b, c, d, e int) {
	k := len(*s) - 1
	entry := (*s)[k]
	*s = (*s)[:k]
	return entry.a, entry.b, entry.c, entry.d, entry.e
}

func (cfg config) trIntroSort(sa, isa []int32, depth, first, last int, budget *budget) {
	s := make(stack, 0, 96)
	var (
		a, b, c int
		v       int32
	)
	trlink := -1
	incr := depth
	limit := ilog2(last - first)
	for {
		switch limit {
		case -1:
			a, b = trPartition(sa, isa[depth-incr:], first, first,
				last, int32(last-1))
			if a < last {
				v = int32(a - 1)
				for c = first; c < a; c++ {
					isa[sa[c]] = v
				}
			}
			if b < last {
				v = int32(b - 1)
				for c = a; c < b; c++ {
					isa[sa[c]] = v
				}
			}
			if b-a > 1 {
				s.push(0, a, b, 0, 0)
				s.push(depth-incr, first, last, -2, trlink)
				trlink = len(s) - 2
			}
			if (a - first) <= (last - b) {
				if (a - first) > 1 {
					s.push(depth, b, last, ilog2(last-b),
						trlink)
					last = a
					limit = ilog2(a - first)
				} else if (last - b) > 1 {
					first = b
					limit = ilog2(last - b)
				} else {
					if len(s) == 0 {
						return
					}
					depth, first, last, limit, trlink =
						s.pop()
				}
			} else {
				if last-b > 1 {
					s.push(depth, first, a, ilog2(a-first),
						trlink)
					first = b
					limit = ilog2(last - b)
				} else if a-first > 1 {
					last = a
					limit = ilog2(a - first)
				} else {
					if len(s) == 0 {
						return
					}
					depth, first, last, limit, trlink =
						s.pop()
				}
			}
			continue
		case -2:
			if len(s) == 0 {
				return
			}
			var dd int
			_, a, b, dd, _ = s.pop()
			if dd == 0 {
				trCopy(sa, isa, first, a, b, last, depth)
			} else {
				if 0 <= trlink {
					s[trlink].d = -1
				}
				trPartialCopy(sa, isa, first, a, b, last, depth)
			}
			if len(s) == 0 {
				return
			}
			depth, first, last, limit, trlink = s.pop()
			continue
		case -3:
			if 0 <= sa[first] {
				a = first
				for {
					isa[sa[a]] = int32(a)
					a++
					if a >= last {
						break
					}
					if sa[a] < 0 {
						break
					}
				}
				first = a
			}
			if first < last {
				a = first
				for {
					sa[a] = ^sa[a]
					a++
					if sa[a] >= 0 {
						break
					}
				}
				var next int
				if isa[sa[a]] != isa[int32(depth)+sa[a]] {
					next = ilog2(a - first + 1)
				} else {
					next = -1
				}
				a++
				if a < last {
					v = int32(a - 1)
					for b = first; b < a; b++ {
						isa[sa[b]] = v
					}
				}

				if budget.check(a - first) {
					if (a - first) <= (last - a) {
						s.push(depth, a, last, -3,
							trlink)
						depth += incr
						last = a
						limit = next
					} else {
						if (last - a) > 1 {
							s.push(depth+incr,
								first, a, next,
								trlink)
							first = a
							limit = -3
						} else {
							depth += incr
							last = a
							limit = next
						}
					}
				} else {
					if trlink >= 0 {
						s[trlink].d = -1
					}
					if (last - a) > 1 {
						first = a
						limit = -3
					} else {
						if len(s) == 0 {
							return
						}
						depth, first, last, limit,
							trlink = s.pop()
					}
				}
			} else {
				if len(s) == 0 {
					return
				}
				depth, first, last, limit, trlink =
					s.pop()
			}
			continue
		}

		if limit < 0 {
			panic(fmt.Errorf("limit=%d negative", limit))
		}

		isaD := isa[depth:]

		if (last - first) <= cfg.trSizeThreshold {
			trInsertionSort(sa[first:last], isaD)
			limit = -3
			continue
		}

		limit--
		if limit < 0 {
			trHeapSort(sa[first:last], isaD)
			limit = -3
			continue
		}

		a = trPivot(sa, isaD, first, last)
		if a != first {
			sa[first], sa[a] = sa[a], sa[first]
		}
		v = isaD[sa[first]]

		a, b = trPartition(sa, isaD, first, first+1, last, v)
		if (last - first) != (b - a) {
			var next int
			if isa[sa[a]] != v {
				next = ilog2(b - a)
			} else {
				next = -1
			}

			v = int32(a - 1)
			for c = first; c < a; c++ {
				isa[sa[c]] = v
			}
			if b < last {
				v = int32(b - 1)
				for c = a; c < b; c++ {
					isa[sa[c]] = v
				}
			}

			if b-a > 1 && budget.check(b-a) {
				if a-first <= last-b {
					if (last - b) <= (b - a) {
						if a-first > 1 {
							s.push(depth+incr, a, b,
								next, trlink)
							s.push(depth, b, last,
								limit, trlink)
							last = a
						} else if last-b > 1 {
							s.push(depth+incr, a, b,
								next, trlink)
							first = b
						} else {
							depth += incr
							first = a
							last = b
							limit = next
						}
					} else if (a - first) <= (b - a) {
						if (a - first) > 1 {
							s.push(depth, b, last,
								limit, trlink)
							s.push(depth+incr, a, b,
								next, trlink)
							last = a
						} else {
							s.push(depth, b, last,
								limit, trlink)
							depth += incr
							first = a
							last = b
							limit = next
						}
					} else {
						s.push(depth, b, last, limit,
							trlink)
						s.push(depth, first, a, limit,
							trlink)
						depth += incr
						first = a
						last = b
						limit = next
					}
				} else {
					if (a - first) <= (b - a) {
						if (last - b) > 1 {
							s.push(depth+incr, a, b,
								next, trlink)
							s.push(depth, first, a,
								limit, trlink)
							first = b
						} else if (a - first) > 1 {
							s.push(depth+incr, a, b,
								next, trlink)
							last = a
						} else {
							depth += incr
							first = a
							last = b
							limit = next
						}
					} else if (last - b) <= (b - a) {
						if (last - b) > 1 {
							s.push(depth, first, a,
								limit, trlink)
							s.push(depth+incr, a, b,
								next, trlink)
							first = b
						} else {
							s.push(depth, first, a,
								limit, trlink)
							depth += incr
							first = a
							last = b
							limit = next
						}
					} else {
						s.push(depth, first, a,
							limit, trlink)
						s.push(depth, b, last,
							limit, trlink)
						depth += incr
						first = a
						last = b
						limit = next
					}
				}
			} else {
				if (b-a) > 1 && trlink >= 0 {
					s[trlink].d = -1
				}
				if (a - first) <= (last - b) {
					if (a - first) > 1 {
						s.push(depth, b, last,
							limit, trlink)
						last = a
					} else if (last - b) > 1 {
						first = b
					} else {
						if len(s) == 0 {
							return
						}
						depth, first, last, limit,
							trlink = s.pop()
					}
				} else {
					if (last - b) > 1 {
						s.push(depth, first, a,
							limit, trlink)
						first = b
					} else if (a - first) > 1 {
						last = a
					} else {
						if len(s) == 0 {
							return
						}
						depth, first, last, limit,
							trlink = s.pop()
					}
				}
			}
		} else {
			if budget.check(last - first) {
				limit = ilog2(last - first)
				depth += incr
			} else {
				if trlink >= 0 {
					s[trlink].d = -1
				}
				if len(s) == 0 {
					return
				}
				depth, first, last, limit, trlink = s.pop()
			}
		}
	}
}

func trMedian3(sa, isaD []int32, v1, v2, v3 int) int {
	y1, y2, y3 := isaD[sa[v1]], isaD[sa[v2]], isaD[sa[v3]]
	if y1 > y2 {
		v1, v2 = v2, v1
		y1, y2 = y2, y1
	}
	if y2 > y3 {
		if y1 > y3 {
			return v1
		}
		return v3
	}
	return v2
}

func trMedian5(sa, isaD []int32, v1, v2, v3, v4, v5 int) int {
	y1, y2, y3, y4, y5 := isaD[sa[v1]], isaD[sa[v2]], isaD[sa[v3]],
		isaD[sa[v4]], isaD[sa[v5]]
	if y2 > y3 {
		v2, v3 = v3, v2
		y2, y3 = y3, y2
	}
	if y4 > y5 {
		v4, v5 = v5, v4
		y4, y5 = y5, y4
	}
	if y2 > y4 {
		v4 = v2
		y4 = y2
		v3, v5 = v5, v3
		y3, y5 = y5, y3
	}
	if y1 > y3 {
		v1, v3 = v3, v1
		y1, y3 = y3, y1
	}
	if y1 > y4 {
		v4 = v1
		y4 = y1
		v3 = v5
		y3 = y5
	}
	if y3 > y4 {
		return v4
	}
	return v3
}

func trPivot(sa, isaD []int32, first, last int) int {
	t := last - first
	middle := first + t/2
	last--

	if t <= 512 {
		if t <= 32 {
			return trMedian3(sa, isaD, first, middle, last)
		}
		t >>= 2
		return trMedian5(sa, isaD, first, first+t, middle,
			last-t, last)
	}

	t >>= 3
	first = trMedian3(sa, isaD, first, first+t, first+(t<<1))
	middle = trMedian3(sa, isaD, middle-t, middle, middle+t)
	last = trMedian3(sa, isaD, last-(t<<1), last-t, last)
	return trMedian3(sa, isaD, first, middle, last)
}

func trPartition(sa, isaD []int32, first, middle, last int, v int32) (ra, rb int) {
	var x int32

	b := middle
	for {
		if b >= last {
			break
		}
		x = isaD[sa[b]]
		if x != v {
			break
		}
		b++
	}
	a := b
	if a < last && x < v {
		for {
			b++
			if b >= last {
				break
			}
			x = isaD[sa[b]]
			if x > v {
				break
			}
			if x == v {
				sa[a], sa[b] = sa[b], sa[a]
				a++
			}
		}
	}
	c := last
	for {
		c--
		if b >= c {
			break
		}
		x = isaD[sa[c]]
		if x != v {
			break
		}
	}
	d := c
	if b < d && x > v {
		for {
			c--
			if b >= c {
				break
			}
			x = isaD[sa[c]]
			if x < v {
				break
			}
			if x == v {
				sa[c], sa[d] = sa[d], sa[c]
				d--
			}
		}
	}

	for b < c {
		sa[b], sa[c] = sa[c], sa[b]
		for {
			b++
			if b >= c {
				break
			}
			x = isaD[sa[b]]
			if x > v {
				break
			}
			if x == v {
				sa[a], sa[b] = sa[b], sa[a]
				a++
			}
		}
		for {
			c--
			if b >= c {
				break
			}
			x = isaD[sa[c]]
			if x < v {
				break
			}
			if x == v {
				sa[c], sa[d] = sa[d], sa[c]
				d--
			}
		}
	}

	if a <= d {
		c = b - 1
		s := a - first
		t := b - a
		if s > t {
			s = t
		}
		e := first
		f := b - s
		for {
			if s <= 0 {
				break
			}
			sa[e], sa[f] = sa[f], sa[e]
			s--
			e++
			f++
		}
		s = d - c
		t = last - d - 1
		if s > t {
			s = t
		}
		e = b
		f = last - s
		for {
			if s <= 0 {
				break
			}
			sa[e], sa[f] = sa[f], sa[e]
			s--
			e++
			f++
		}
		first += b - a
		last -= d - c
	}
	return first, last
}

func trInsertionSort(sa []int32, isaD []int32) {
	for i := 1; i < len(sa); i++ {
		var r int32
		j := i - 1
		s, t := sa[j], sa[i]
		u := isaD[t]
	loop:
		for {
			r = u - isaD[s]
			if r >= 0 {
				if r == 0 {
					sa[j] = ^s
				}
				break loop
			}
			for {
				j--
				if j < 0 {
					break loop
				}
				if s = sa[j]; s >= 0 {
					continue loop
				}
			}
		}
		if j++; j < i {
			copy(sa[j+1:], sa[j:i])
			sa[j] = t
		}
	}
}

func trHeapSort(sa []int32, isaD []int32) {
	if len(sa) < 2 {
		return
	}
	var i, j int
	var k int32
	l, r := len(sa)>>1, len(sa)-1

	for {
		// H2
		if l > 0 {
			l--
			k = sa[l]
		} else {
			k = sa[r]
			sa[r] = sa[0]
			r--
			if r == 0 {
				sa[0] = k
				break
			}
		}

		// H3
		j = l

		y := isaD[k]
		for {
			// H4
			i, j = j, (j<<1)+1
			if j > r {
				break
			}

			// H5
			p := sa[j]
			u := isaD[p]
			if j < r {
				q := sa[j+1]
				v := isaD[q]
				if u < v {
					j++
					p, u = q, v
				}

			}

			// H6
			if y >= u {
				break
			}

			// H7
			sa[i] = p
		}

		// H8
		sa[i] = k
	}

	// lower equal values must be bitwise negated.
	k = sa[0]
	x := isaD[k]
	for i, l := range sa[1:] {
		y := isaD[l]
		if x == y {
			sa[i] = ^k
		}
		k, x = l, y
	}
}

func trCopy(sa, isa []int32, first, a, b, last, depth int) {
	v := int32(b - 1)
	d := a - 1
	var c int
	for c = first; c <= d; c++ {
		s := sa[c] - int32(depth)
		if 0 <= s && isa[s] == v {
			d++
			sa[d] = s
			isa[s] = int32(d)
		}
	}
	c = last - 1
	e := d + 1
	d = b
	for ; e < d; c-- {
		s := sa[c] - int32(depth)
		if 0 <= s && isa[s] == v {
			d--
			sa[d] = s
			isa[s] = int32(d)
		}
	}
}

func trPartialCopy(sa, isa []int32, first, a, b, last, depth int) {
	v := int32(b - 1)
	newrank := int32(-1)
	lastrank := int32(-1)
	d := a - 1
	var c int
	for c = first; c <= d; c++ {
		s := sa[c] - int32(depth)
		if 0 <= s && isa[s] == v {
			d++
			sa[d] = s
			rank := isa[s+int32(depth)]
			if lastrank != rank {
				lastrank = rank
				newrank = int32(d)
			}
			isa[s] = newrank
		}
	}

	lastrank = -1
	var e int
	for e = d; first <= e; e-- {
		rank := isa[sa[e]]
		if lastrank != rank {
			lastrank = rank
			newrank = int32(e)
		}
		if newrank != rank {
			isa[sa[e]] = newrank
		}
	}
}

type budget struct {
	chance int
	remain int
	incval int
	count  int
}

func (b *budget) check(size int) bool {
	if size <= b.remain {
		b.remain -= size
		return true
	}
	if b.chance == 0 {
		b.count += size
		return false
	}
	b.remain += b.incval - size
	b.chance--
	return true
}
