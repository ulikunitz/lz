package lz

import (
	"fmt"
	"math"
	"regexp"
	"sync"
)

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
	table []Entry
	mask  uint64
	shift uint

	inputLen int
	hashBits int
}

func (h *hash) String() string {
	return fmt.Sprintf("hash_%d:%d", h.inputLen, h.hashBits)
}

func verifyHashParams(inputLen, hashBits int) error {
	if !(2 <= inputLen && inputLen <= 8) {
		return fmt.Errorf("lz: invalid inputLen=%d; must be 2..8",
			inputLen)
	}
	maxHashBits := min(24, 8*inputLen)
	if !(0 <= hashBits && hashBits <= maxHashBits) {
		return fmt.Errorf(
			"lz: invalid hashBits=%d; must be in range 0..%d",
			hashBits, maxHashBits)
	}
	return nil
}

var hashRegexp = sync.OnceValue(func() *regexp.Regexp {
	return regexp.MustCompile(`^hash_\d+:\d+$`)
})

func parseHashName(name string) (inputLen, hashBits int, err error) {
	if !hashRegexp().MatchString(name) {
		return 0, 0, fmt.Errorf(
			"lz: invalid hash name %q; must be in format hash-<inputLen>:<hashBits>",
			name)
	}
	if _, err = fmt.Sscanf(name, "hash_%d:%d", &inputLen, &hashBits); err != nil {
		return 0, 0, fmt.Errorf(
			"lz: invalid hash name %q; must be in format hash-<inputLen>:<hashBits>",
			name)
	}
	err = verifyHashParams(inputLen, hashBits)
	return inputLen, hashBits, err
}

func newHash(inputLen, hashBits int) (*hash, error) {
	if err := verifyHashParams(inputLen, hashBits); err != nil {
		return nil, err
	}
	h := &hash{
		table:    make([]Entry, 1<<hashBits),
		inputLen: inputLen,
		hashBits: hashBits,
		mask:     1<<(inputLen*8) - 1,
		shift:    64 - uint(hashBits),
	}
	return h, nil
}

func (h *hash) InputLen() int { return h.inputLen }

// Reset clears the hash table.
func (h *hash) Reset() {
	for i := range h.table {
		h.table[i] = Entry{}
	}
}

// Shift  removes delta from all positions in the hash table. Entries with
// positions smaller than delta will be cleared.
func (h *hash) Shift(delta int) {
	if delta <= 0 {
		return
	}
	if delta > math.MaxUint32 {
		panic("lz: delta too large")
	}
	d := uint32(delta)
	for i, e := range h.table {
		if e.i < d {
			h.table[i] = Entry{}
		} else {
			h.table[i].i = e.i - d
		}
	}
}

func (h *hash) Put(p []byte, a, w int) int {
	b := min(w, max(len(p)-max(h.inputLen, 4)+1, 0))
	_p := p[:b+7]
	for i := a; i < b; i++ {
		v := _getLE64(_p[i:])
		h.table[hashValue(v&h.mask, h.shift)] =
			Entry{i: uint32(i), v: uint32(v)}
	}
	return w - b
}

func (h *hash) Get(v uint64) []Entry {
	v &= h.mask
	i := hashValue(v, h.shift)
	r := h.table[i : i+1]
	e := r[0]
	if e.v&uint32(h.mask) != uint32(v) || (e == Entry{}) {
		return nil
	}
	return r
}
