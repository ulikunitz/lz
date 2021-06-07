package lz

import (
	"fmt"
	"math/bits"
	"strings"
)

const bsMask = 1<<6 - 1

type lbitset struct {
	a []uint64
	n int
}

func (b *lbitset) init(n int) {
	k := (n + 63) / 64
	if k <= cap(b.a) {
		b.a = b.a[:k]
		for i := range b.a {
			b.a[i] = 0
		}
	} else {
		b.a = make([]uint64, k)
	}
	b.n = n
}

func (b *lbitset) clear() {
	for i := range b.a {
		b.a[i] = 0
	}
}

func (b *lbitset) insert(i int) {
	b.a[i>>6] |= 1 << uint(i&bsMask)
}

func (b *lbitset) isMember(i int) bool {
	return (b.a[i>>6] & (1 << uint(i&bsMask))) != 0
}

func (b *lbitset) memberBefore(i int) (k int, ok bool) {
	m := uint64(1)<<uint(i&bsMask) - 1
	i >>= 6
	k = 63 - bits.LeadingZeros64(b.a[i]&m)
	for {
		if k >= 0 {
			return i<<6 + k, true
		}
		i--
		if i < 0 {
			return 0, false
		}
		k = 63 - bits.LeadingZeros64(b.a[i])
	}
}

func (b *lbitset) memberAfter(i int) (k int, ok bool) {
	i++
	if i >= b.n {
		return 0, false
	}
	m := ^(uint64(1)<<uint(i&bsMask) - 1)
	i >>= 6
	k = bits.TrailingZeros64(b.a[i] & m)
	for {
		if k < 64 {
			return i<<6 + k, true
		}
		i++
		if i >= len(b.a) {
			return 0, false
		}
		k = bits.TrailingZeros64(b.a[i])
	}
}

func (b *lbitset) pop() int {
	n := 0
	for _, x := range b.a {
		n += bits.OnesCount64(x)
	}
	return n
}

type xset []uint64

func (s *xset) clear() *xset {
	*s = (*s)[:0]
	return s
}

func (s *xset) ensureLen(k int) *xset {
	a := *s
	if k <= len(a) {
		return s
	}
	if k <= cap(a) {
		n := len(a)
		a = a[:k]
		b := a[n:]
		for i := range b {
			b[i] = 0
		}
		*s = a
		return s
	}
	b := make([]uint64, k)
	copy(b, a)
	*s = b
	return s
}

func (s *xset) insert(i ...int) *xset {
	a := *s
	for _, j := range i {
		k := j >> 6
		if k >= len(a) {
			a = *(s.ensureLen(k + 1))
		}
		a[k] |= 1 << uint(j&bsMask)
	}
	return s
}

func (s *xset) isMember(i int) bool {
	a := *s
	k := i >> 6
	if k >= len(a) {
		return false
	}
	return (a[k] & (1 << uint(i&bsMask))) != 0
}

func (s *xset) memberAfter(i int) (k int, ok bool) {
	a := *s
	i++
	j := i >> 6
	if j >= len(a) {
		return 0, false
	}
	m := ^(uint64(1)<<uint(i&bsMask) - 1)
	k = bits.TrailingZeros64(a[j] & m)
	for {
		if k < 64 {
			return j<<6 + k, true
		}
		j++
		if j >= len(a) {
			return 0, false
		}
		k = bits.TrailingZeros64(a[j])
	}
}

func (s *xset) firstMember() (k int, ok bool) {
	a := *s
	for j := 0; j < len(a); j++ {
		k = bits.TrailingZeros64(a[j])
		if k < 64 {
			return j<<6 + k, true
		}
	}
	return 0, false
}

func (s *xset) assign(u *xset) *xset {
	if s == u {
		return s
	}
	s.ensureLen(len(*u))
	*s = (*s)[:len(*u)]
	copy(*s, *u)
	return s
}

func (s *xset) intersect(u, v *xset) *xset {
	s.assign(u)
	if u == v {
		return s
	}
	a, b := *s, *v
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i, v := range b[:n] {
		a[i] &= v
	}
	for n--; n >= 0 && a[n] == 0; n-- {
	}
	n++
	*s = a[:n]
	return s
}

func (s *xset) slice() []int {
	var a []int
	for i, x := range *s {
		j := i << 6
		for {
			k := bits.TrailingZeros64(x)
			if k >= 64 {
				break
			}
			j += k
			a = append(a, j)
			x >>= k + 1
			j++
		}
	}
	return a
}

func (s *xset) String() string {
	var sb strings.Builder
	sb.WriteByte('{')
	for i, k := range s.slice() {
		if i > 0 {
			sb.WriteString(", ")
		}
		fmt.Fprint(&sb, k)
	}
	sb.WriteByte('}')
	return sb.String()
}
