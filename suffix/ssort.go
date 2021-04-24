package suffix

import (
	"bytes"
	"math/bits"
)

// TODO: rewrite heapSort so that it negates equal elements

type ssorter struct {
	a []int32
	t []byte
	p []int32
	config
}

func (cfg config) newSsorter(a []int32, t []byte, p []int32) *ssorter {
	s := &ssorter{
		a:      a,
		t:      t,
		p:      p,
		config: cfg,
	}

	return s
}

// sorts substrings in a[f:b]. The argument lastIndex indicates that the f
// refers to the last B* substring.
func (s *ssorter) ssort(f, b int, lastIndex bool) {
	i := f
	if lastIndex {
		i++
	}
	s.introSort(i, b)

	if !lastIndex || i == b {
		return
	}

	// The first element refers to the last B* substring in the text. We
	// have to put in the right place.
	t := s.a[f]
	x := s.t[s.p[t]+2:]
loop:
	for i < b {
		k := (i + b) >> 1
		r := bytes.Compare(x, s.substringNeg(k, 2))
		switch {
		case r < 0:
			b = k
		case r == 0:
			t = ^t
			b = k + 1
			break loop
		default:
			i = k + 1
		}
	}
	copy(s.a[f:], s.a[f+1:b])
	s.a[b-1] = t
}

func (s *ssorter) Len() int { return len(s.a) }

func (s *ssorter) Swap(i, j int) {
	s.a[i], s.a[j] = s.a[j], s.a[i]
}

// substring returns the B* substring starting at offset d. The method assumes
// that d is non-negative.
func (s *ssorter) substring(i, d int) []byte {
	k := s.a[i]
	b := s.p[k+1] + 2
	f := s.p[k] + int32(d)
	if f >= b {
		return nil
	}
	return s.t[f:b]
}

func (s *ssorter) substringK(k int32, d int) []byte {
	b := s.p[k+1] + 2
	f := s.p[k] + int32(d)
	if f >= b {
		return nil
	}
	return s.t[f:b]
}

func (s *ssorter) Less(i, j int) bool {
	x, y := s.substring(i, 2), s.substring(j, 2)
	return bytes.Compare(x, y) < 0
}

func (s *ssorter) LessD(i, j, d int) bool {
	x, y := s.substring(i, d), s.substring(j, d)
	return bytes.Compare(x, y) < 0
}

func absNeg(x int32) int32 {
	return x ^ (x >> 31)
}

// substring returns the B* substring starting at offset d. The method assumes
// that d is non-negative. The function can tolerate negated a entries.
func (s *ssorter) substringNeg(i, d int) []byte {
	k := absNeg(s.a[i])
	b := s.p[k+1] + 2
	f := s.p[k] + int32(d)
	if f >= b {
		return nil
	}
	return s.t[f:b]
}

func (s *ssorter) insertSortNeg(f, b, d int) {
	// TODO: rework using trInsetionSort as example; we might not
	// need substringNeg
	for i := f + 1; i < b; i++ {
		y := s.substringNeg(i, d)
		var j int
	innerLoop:
		for j = i - 1; j >= f; j-- {
			x := s.substringNeg(j, d)
			r := bytes.Compare(x, y)
			switch {
			case r < 0:
				break innerLoop
			case r == 0:
				s.a[i] = ^s.a[i]
				break innerLoop
			}
		}
		j++
		if j == i {
			continue
		}
		c := s.a[i]
		copy(s.a[j+1:], s.a[j:i])
		s.a[j] = c
	}
}

func (s *ssorter) heapSort(f, b, d int) {
	// TODO: Don't use insertSortNeg, but run your own loop for fixing
	// output.
	a := s.a[f:b]
	if len(a) < 2 {
		return
	}
	var i, j int
	var k int32
	var x, u, v []byte
	l, r := len(a)>>1, len(a)-1

	for {
		// H2
		if l > 0 {
			l--
			k = a[l]
		} else {
			k = a[r]
			a[r] = a[0]
			r--
			if r == 0 {
				a[0] = k
				s.insertSortNeg(f, b, d)
				return
			}
		}

		// H3
		j = l
		x = s.substringK(k, d)

		for {
			// H4
			i, j = j, (j<<1)+1
			if j > r {
				break
			}
			u = s.substringK(a[j], d)
			// H5
			if j < r {
				v = s.substringK(a[j+1], d)
				if bytes.Compare(u, v) < 0 {
					j++
					u = v
				}
			}
			// H6
			if bytes.Compare(x, u) >= 0 {
				break
			}
			// H7
			a[i] = a[j]
		}

		// H8
		a[i] = k
	}
}

func (s *ssorter) medianOf3(i, j, k, d int) int {
	td := s.t[d:]
	a, b, c := td[s.p[s.a[i]]], td[s.p[s.a[j]]], td[s.p[s.a[k]]]
	if b > c {
		j, k = k, j
		b, c = c, b
	}
	if a <= b {
		return j
	}
	if a <= c {
		return i
	}
	return k
}

func (s *ssorter) exchange(i, j, k int) {
	if j-i > k-j {
		j = i + k - j
	}
	for ; i < j; i++ {
		k--
		s.a[i], s.a[k] = s.a[k], s.a[i]
	}
}

// partition splits the area between f and b in three sections, less than the
// pivot, equal to the pivot, greater than the pivot. The returned flag eos
// signals if the equal section is at end of the substring.
func (s *ssorter) partition(f, b, m, d int) (e, g int) {
	if f < m {
		s.a[f], s.a[m] = s.a[m], s.a[f]
	}
	td := s.t[d:]
	p := td[s.p[s.a[f]]]
	u, v := f+1, b
	i, j := f+1, b
burn:
	for {
		for ; ; i++ {
			if i >= j {
				break burn
			}
			q := td[s.p[s.a[i]]]
			if q > p {
				break
			}
			if q < p {
				continue
			}
			s.a[u], s.a[i] = s.a[i], s.a[u]
			u++
		}
		j--
		for ; ; j-- {
			for i >= j {
				break burn
			}
			q := td[s.p[s.a[j]]]
			if q < p {
				break
			}
			if q > p {
				continue
			}
			v--
			s.a[j], s.a[v] = s.a[v], s.a[j]
		}
		s.a[i], s.a[j] = s.a[j], s.a[i]
		i++
	}
	s.exchange(f, u, i)
	s.exchange(i, v, b)
	return f + i - u, b - (v - i)
}

func ilog2(x int) int {
	return 63 - bits.LeadingZeros64(uint64(x))
}

func (s *ssorter) introSort(f, b int) {
	s.introSortLoop(f, b, 2, 2*ilog2(b-f))
}

func (s *ssorter) introSortLoop(f, b, d, depthLimit int) {
	for b-f > s.sizeThreshold {
		if depthLimit <= 0 {
			s.heapSort(f, b, d)
			return
		}
		m := s.medianOf3(f, f+(b-f)/2, b-1, d)
		e, g := s.partition(f, b, m, d)
		depthLimit--
		s.introSortLoop(f, e, d, depthLimit)
		// if the equal section is at end of substring, we don't need to
		// go further
		e = s.filterStringEnds(e, g, d+1)
		s.introSortLoop(e, g, d+1, depthLimit)
		f = g
	}
	s.insertSortNeg(f, b, d)
}

func (s *ssorter) filterStringEnds(f, b, d int) int {
	if d >= 4 {
		i := f
		// burn the candle from both ends
	burn:
		for {
			for {
				if f >= b {
					break burn
				}
				k := s.a[f]
				if s.p[k]+int32(d) < s.p[k+1]+2 {
					break
				}
				s.a[f] = ^k
				f++
			}
			b--
			for {
				if f >= b {
					break burn
				}
				if k := s.a[b]; s.p[k]+int32(d) >= s.p[k+1]+2 {
					break
				}
				b--
			}
			s.a[f], s.a[b] = ^s.a[b], s.a[f]
			f++
		}

		if f > i {
			s.a[i] = ^s.a[i]
		}
	}
	return f
}
