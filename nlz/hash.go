package nlz

import (
	"fmt"
	"math"
)

// prime is used by [hashValue].
const prime = 9920624304325388887

// hashValue computes a hash from the string stored in x with the first byte
// stored on the lowest bits. The shift values ensures that only 64 - shift bits
// potential non-zero bits remain.
func hashValue(x uint64, shift uint) uint32 {
	return uint32((x * prime) >> shift)
}

// hashEntry is used for hashEntry. The value field allows a fast check whether
// a match has been found, which is cache-optimized.
type hashEntry struct {
	pos   uint32
	value uint32
}

// The hash implements a match finder and can be directly used in a parser.
type hash struct {
	table    []hashEntry
	mask     uint64
	shift    uint
	inputLen int
}

// init initializes the hash structure.
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

// reset clears the hash table.
func (h *hash) reset() {
	for i := range h.table {
		h.table[i] = hashEntry{}
	}
}

// shiftOffsets removes delta from all positions in the hash table. Entries with
// positions smaller than delta will be cleared.
func (h *hash) shiftOffsets(delta uint32) {
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

type HashOptions struct {
	InputLen int
	HashBits int

	WindowSize   int
	MinMatchSize int
	MaxMatchSize int
}

func (opt *HashOptions) SetDefaults() {
	if opt.InputLen == 0 {
		opt.InputLen = 4
	}
	if opt.HashBits == 0 {
		opt.HashBits = 16
	}
	if opt.WindowSize == 0 {
		opt.WindowSize = 32 << 10
	}
	if opt.MinMatchSize == 0 {
		opt.MinMatchSize = 2
	}
	if opt.MaxMatchSize == 0 {
		opt.MaxMatchSize = 273
	}
}

func (opt *HashOptions) Verify() error {
	if !(2 <= opt.InputLen && opt.InputLen <= 8) {
		return fmt.Errorf("lz: InputLen must be in range [2,8]")
	}
	maxHashBits := 24
	if t := 8 * opt.InputLen; t < maxHashBits {
		maxHashBits = t
	}
	if !(0 <= opt.HashBits && opt.HashBits <= maxHashBits) {
		return fmt.Errorf("lz: hashBits=%d; must be <= %d",
			opt.HashBits, maxHashBits)
	}
	if !(0 <= opt.WindowSize && int64(opt.WindowSize) <= math.MaxUint32) {
		return fmt.Errorf("lz: WindowSize=%d; must be in range [0..%d]",
			opt.WindowSize, math.MaxUint32)
	}
	if !(2 <= opt.MinMatchSize && opt.MinMatchSize <= opt.MaxMatchSize) {
		return fmt.Errorf("lz: MinMatchSize=%d; must be in range [2..MaxMatchSize=%d]",
			opt.MinMatchSize, opt.MaxMatchSize)
	}
	return nil
}
