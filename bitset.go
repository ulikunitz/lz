package lz

import "math/bits"

const bsMask = 1<<6 - 1

type bitset struct {
	a []uint64
	n int
}

func (b *bitset) init(n int) {
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

func (b *bitset) clear() {
	for i := range b.a {
		b.a[i] = 0
	}
}

func (b *bitset) insert(i int) {
	b.a[i>>6] |= 1 << uint(i&bsMask)
}

func (b *bitset) isMember(i int) bool {
	return (b.a[i>>6] & (1 << uint(i&bsMask))) != 0
}

func (b *bitset) memberBefore(i int) (k int, ok bool) {
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

func (b *bitset) memberAfter(i int) (k int, ok bool) {
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

func (b *bitset) pop() int {
	n := 0
	for _, x := range b.a {
		n += bits.OnesCount64(x)
	}
	return n
}
