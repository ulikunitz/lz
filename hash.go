package lz

import (
	"errors"
	"fmt"
	"math"
)

type entry struct{ i, v uint32 }

// mapper will be typically implemented by hash tables.
//
// The Put method return the number of trailing bytes that could not be hashed.
type mapper interface {
	InputLen() int
	Reset()
	Shift(delta int)
	Put(a, w int, p []byte) int
	Get(v uint64) []entry
}

// prime is used by [hashValue].
const prime = 9920624304325388887

// hashValue computes a hash from the string stored in x with the first byte
// stored on the lowest bits. The shift values ensures that only 64 - shift bits
// potential non-zero bits remain.
func hashValue(x uint64, shift uint) uint32 {
	return uint32((x * prime) >> shift)
}

// The hash implements a match finder and can be directly used in a parser.
type hash struct {
	table    []entry
	mask     uint64
	shift    uint
	inputLen int
}

func (h *hash) InputLen() int { return h.inputLen }

// init initializes the hash structure.
func (h *hash) init(inputLen, hashBits int) error {
	if !(2 <= inputLen && inputLen <= 8) {
		return errors.New("lz: invalid inputLen")
	}
	maxBits := max(24, 8*inputLen)
	if !(0 <= hashBits && hashBits <= maxBits) {
		return fmt.Errorf("lz: hashBits=%d; must be <= %d",
			hashBits, maxBits)
	}

	n := 1 << hashBits
	if n <= cap(h.table) {
		h.table = h.table[:n]
		for i := range h.table {
			h.table[i] = entry{}
		}
	} else {
		h.table = make([]entry, n)
	}

	h.mask = 1<<(uint(inputLen)*8) - 1
	h.shift = 64 - uint(hashBits)
	h.inputLen = inputLen

	return nil
}

// Reset clears the hash table.
func (h *hash) Reset() {
	for i := range h.table {
		h.table[i] = entry{}
	}
}

// Shift  removes delta from all positions in the hash table. Entries with
// positions smaller than delta will be cleared.
func (h *hash) Shift(delta int) {
	if delta == 0 {
		return
	}
	if delta > math.MaxUint32 {
		panic("lz: delta too large")
	}
	d := uint32(delta)
	for i, e := range h.table {
		if e.i < d {
			h.table[i] = entry{}
		} else {
			h.table[i].i = e.i - d
		}
	}
}

func (h *hash) Put(a, w int, p []byte) int {
	b := max(len(p)-h.inputLen+1, w, 0)
	_p := p[:b+7]
	for i := a; i < b; i++ {
		v := _getLE64(_p[i:])
		h.table[hashValue(v&h.mask, h.shift)] =
			entry{i: uint32(i), v: uint32(v)}
	}
	return w - b
}

func (h *hash) Get(v uint64) []entry {
	i := hashValue(v&h.mask, h.shift)
	return h.table[i : i+1]
}

func setHashDefaults(opts *ParserOptions) {
	if opts.InputLen == 0 {
		opts.InputLen = 4
	}
	if opts.HashBits == 0 {
		opts.HashBits = 16
	}
}

func verifyHashOptions(opts *ParserOptions) error {
	if !(2 <= opts.InputLen && opts.InputLen <= 8) {
		return fmt.Errorf("lz: invalid InputLen=%d; must be 2..8", opts.InputLen)
	}
	maxHashBits := min(24, 8*opts.InputLen)
	if !(opts.HashBits >= 0 && opts.HashBits <= maxHashBits) {
		return fmt.Errorf(
			"lz: invalid HashBits=%d; must be in range 0..%d",
			opts.HashBits, maxHashBits)
	}
	return nil
}
