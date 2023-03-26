package lz

import (
	"fmt"
	"reflect"
)

// hashEntry is used for hashEntry. The value field allows a fast check whether
// a match has been found, which is cache-optimized.
type hashEntry struct {
	pos   uint32
	value uint32
}

type hash struct {
	table    []hashEntry
	mask     uint64
	shift    uint
	inputLen int
}

func (h *hash) additionalMemSize() uintptr {
	return uintptr(cap(h.table)) * reflect.TypeOf(hashEntry{}).Size()
}

func (h *hash) init(inputLen, hashBits int) error {
	if !(2 <= inputLen && inputLen <= 8) {
		return fmt.Errorf("lz: inputLen must be in range [2,8]")
	}
	maxHashBits := 24
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
			h.table[i] = hashEntry{}
		}
	} else {
		h.table = make([]hashEntry, n)
	}

	h.mask = 1<<(uint(inputLen)*8) - 1
	h.shift = 64 - uint(hashBits)
	h.inputLen = inputLen

	return nil
}

func (h *hash) reset() {
	for i := range h.table {
		h.table[i] = hashEntry{}
	}
}

func (h *hash) adapt(delta uint32) {
	if delta == 0 {
		return
	}
	for i, e := range h.table {
		if e.pos < delta {
			h.table[i] = hashEntry{}
		} else {
			h.table[i].pos = e.pos - delta
		}
	}
}

func (h *hash) add(pos uint32, x uint64) {
	x &= h.mask
	e := &h.table[h.hashValue(x)]
	e.pos = pos
	e.value = uint32(x)
}

func (h *hash) appendMatchAndAdd(matches []uint32, pos uint32, x uint64) []uint32 {
	x &= h.mask
	y := uint32(x)
	e := &h.table[h.hashValue(x)]
	if e.value == y {
		matches = append(matches, e.pos)
	}
	e.pos = pos
	e.value = y
	return matches 
}

// prime is used for hashing
const prime = 9920624304325388887

func (h *hash) hashValue(x uint64) uint32 {
	return uint32((x * prime) >> h.shift)
}
