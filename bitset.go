// SPDX-FileCopyrightText: © 2021 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

package lz

import (
	"fmt"
	"math/bits"
	"strings"
)

const bsMask = 1<<6 - 1

type bitset struct {
	// words of a
	a []uint64
	// number of zero words before a starts
	off int
}

/*
func (b *bitset) normalize() {
	var i, j int
	var x uint64
	nonZero := false
	for i, x = range b.a {
		if x != 0 {
			nonZero = true
			break
		}
	}
	if !nonZero {
		i = len(b.a)
	}
	for j = len(b.a) - 1; j >= i; j-- {
		if b.a[j] != 0 {
			break
		}
	}
	j++
	if 0 < i {
		a := b.a[:j-i]
		copy(a, b.a[i:])
		b.a = a
		b.off += i
	} else {
		b.a = b.a[:j]
	}
}
*/

func (b *bitset) support(min, max int) *bitset {
	kmin, kmax := min>>6, max>>6
	if b.off <= kmin && kmax < b.off+len(b.a) {
		return b
	}
	var y bitset
	d := 0
	if len(b.a) == 0 {
		b.off = kmin
		y.off = kmin
	} else if kmin < b.off {
		y.off = kmin
		d = b.off - y.off
	} else {
		y.off = b.off
	}
	n := kmax + 1 - y.off
	if n < d+len(b.a) {
		n = d + len(b.a)
	}
	if n > cap(b.a) {
		y.a = make([]uint64, n)
		copy(y.a[d:], b.a)
	} else {
		y.a = b.a[:n]
		for i := range y.a[:d] {
			y.a[i] = 0
		}
		d += copy(y.a[d:], b.a)
		t := y.a[d:]
		for i := range t {
			t[i] = 0
		}
	}
	*b = y
	return b
}

func (b *bitset) insert(i ...int) *bitset {
	if len(i) == 0 {
		return b
	}
	min := i[0]
	max := min
	for _, j := range i[1:] {
		if j < min {
			min = j
		}
		if j > max {
			max = j
		}
	}
	if min < 0 {
		panic("lz: negative arguments to bitset insert")

	}
	b.support(min, max)
	for _, j := range i {
		k := j>>6 - b.off
		b.a[k] |= 1 << uint(j&63)
	}
	return b
}

/*
func (b *bitset) delete(i int) *bitset {
	k := i>>6 - b.off
	if !(0 <= k && k < len(b.a)) {
		// nothing todo
		return b
	}
	b.a[k] &^= 1 << uint(i&63)
	if k == 0 || k+1 == len(b.a) {
		b.normalize()
	}
	return b
}
*/

func (b *bitset) clear() {
	b.a = b.a[:0]
	b.off = 0
}

/*
func (b *bitset) member(i int) bool {
	k := (i >> 6) - b.off
	if !(0 <= k && k < len(b.a)) {
		return false
	}
	return b.a[k]&(1<<uint(i&63)) != 0
}
*/

/*
func (b *bitset) pop() int {
	n := 0
	for _, x := range b.a {
		n += bits.OnesCount64(x)
	}
	return n
}
*/

func (b *bitset) memberBefore(i int) (j int, ok bool) {
	k := i>>6 - b.off
	if k < 0 {
		return -1, false
	}
	if k < len(b.a) {
		m := uint64(1)<<uint(i&63) - 1
		j = 63 - bits.LeadingZeros64(b.a[k]&m)
	} else {
		k = len(b.a)
		j = -1
	}
	for {
		if j >= 0 {
			return (b.off+k)<<6 + j, true
		}
		k--
		if k < 0 {
			return -1, false
		}
		j = 63 - bits.LeadingZeros64(b.a[k])
	}
}

func (b *bitset) memberAfter(i int) (j int, ok bool) {
	i++
	k := i>>6 - b.off
	if k >= len(b.a) {
		return -1, false
	}
	if k >= 0 {
		m := ^(uint64(1)<<uint(i&bsMask) - 1)
		j = bits.TrailingZeros64(b.a[k] & m)
	} else {
		k = -1
		j = 64
	}
	for {
		if j < 64 {
			return (b.off+k)<<6 + j, true
		}
		k++
		if k >= len(b.a) {
			return -1, false
		}
		j = bits.TrailingZeros64(b.a[k])
	}
}

/*
func (b *bitset) firstMember() (j int, ok bool) {
	if len(b.a) == 0 {
		return -1, false
	}
	j = bits.TrailingZeros64(b.a[0])
	if j >= 64 {
		panic("b is not normalized")
	}
	return (b.off << 6) + j, true
}
*/

/*
func (b *bitset) clone(u *bitset) *bitset {
	if b == u {
		return b
	}
	b.off = u.off
	if len(u.a) > cap(b.a) {
		b.a = make([]uint64, len(u.a))
	} else {
		b.a = b.a[:len(u.a)]
	}
	copy(b.a, u.a)
	return b
}
*/

/*
func (b *bitset) intersect(u, v *bitset) *bitset {
	if u == v {
		return b.clone(u)
	}
	if u.off > v.off {
		u, v = v, u
	}
	s, t := u.off+len(u.a), v.off+len(v.a)
	if s <= v.off {
		b.off = 0
		b.a = b.a[:0]
		return b
	}
	var y bitset
	y.off = v.off
	n := min(s, t) - y.off
	if b == u || b == v || n > cap(b.a) {
		y.a = make([]uint64, n)
	} else {
		y.a = b.a[:n]
	}
	p, q := u.a[y.off-u.off:], v.a[y.off-v.off:]
	for k := range y.a {
		y.a[k] = p[k] & q[k]
	}
	y.normalize()
	*b = y
	return b
}
*/

func (b *bitset) slice() []int {
	var s []int
	for k, x := range b.a {
		base := (b.off + k) << 6
		for x != 0 {
			i := bits.TrailingZeros64(x)
			s = append(s, base+i)
			x &^= 1 << uint(i)
		}
	}
	return s
}

func (b *bitset) String() string {
	var sb strings.Builder
	sb.WriteByte('{')
	for i, j := range b.slice() {
		if i > 0 {
			sb.WriteString(", ")
		}
		fmt.Fprint(&sb, j)
	}
	sb.WriteByte('}')
	return sb.String()
}
