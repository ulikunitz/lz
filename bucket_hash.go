package lz

import (
	"fmt"
	"reflect"
)

type bucketEntry struct {
	pos uint32
	val uint32
}

// bucket is used for bucket. The value field allows a fast check whether
// a match has been found, which is cache-optimized.
type bucket struct {
	table [32]bucketEntry
	i byte
}

func (b *bucket) add(pos, val uint32) {
	b.table[b.i] = bucketEntry{pos: pos, val: val}
	b.i++
	if int(b.i) >= len(b.table) {
		b.i = 0
	}
}

func (b *bucket) adapt(delta uint32) {
	if delta == 0 {
		return
	}
	for i, e := range b.table {
		if e.pos < delta {
			b.table[i] = bucketEntry{}
		} else {
			b.table[i].pos = e.pos - delta
		}
	}
}

type bucketHash struct {
	table    []bucket
	mask     uint64
	shift    uint
	inputLen int
}

func (h *bucketHash) additionalMemSize() uintptr {
	return uintptr(cap(h.table)) * reflect.TypeOf(bucket{}).Size()
}

func (h *bucketHash) init(inputLen, hashBits int) error {
	if !(2 <= inputLen && inputLen <= 8) {
		return fmt.Errorf("lz: inputLen must be in range [2,8]")
	}
	maxHashBits := 32
	if t := 8 * inputLen; t < maxHashBits {
		maxHashBits = t
	}
	if !(0 <= hashBits && hashBits <= maxHashBits) {
		return fmt.Errorf("lz: hashBits=%d; must be <= %d",
			hashBits, maxHashBits)
	}

	n := 1 << hashBits
	if n <= cap(h.table) {
		h.table = h.table[:n]
		for i := range h.table {
			h.table[i] = bucket{}
		}
	} else {
		h.table = make([]bucket, n)
	}

	h.mask = 1<<(uint(inputLen)*8) - 1
	h.shift = 64 - uint(hashBits)
	h.inputLen = inputLen

	return nil
}

func (h *bucketHash) reset() {
	for i := range h.table {
		h.table[i] = bucket{}
	}
}

func (h *bucketHash) adapt(delta uint32) {
	if delta == 0 {
		return
	}
	for i := range h.table {
		h.table[i].adapt(delta)
	}
}

func (h *bucketHash) hashValue(x uint64) uint32 {
	return uint32((x * prime) >> h.shift)
}
