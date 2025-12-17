package lz

import (
	"encoding/json"
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

// The hash implements a match finder and can be directly used in a parser.
type hash struct {
	table    []Entry
	mask     uint64
	shift    uint
	inputLen int
}

func (h *hash) InputLen() int { return h.inputLen }

// init initializes the hash structure.
func (h *hash) init(hopts HashOptions) error {
	if err := hopts.verify(); err != nil {
		return err
	}
	hashBits := hopts.HashBits
	inputLen := hopts.InputLen

	n := 1 << hashBits
	if n <= cap(h.table) {
		h.table = h.table[:n]
		for i := range h.table {
			h.table[i] = Entry{}
		}
	} else {
		h.table = make([]Entry, n)
	}

	h.mask = 1<<(uint(inputLen)*8) - 1
	h.shift = 64 - uint(hashBits)
	h.inputLen = inputLen

	return nil
}

// Reset clears the hash table.
func (h *hash) Reset() {
	for i := range h.table {
		h.table[i] = Entry{}
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
			h.table[i] = Entry{}
		} else {
			h.table[i].i = e.i - d
		}
	}
}

func (h *hash) Put(a, w int, p []byte) int {
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

// HashOptions provides the parameters for the Hash Mapper.
type HashOptions struct {
	InputLen int
	HashBits int
}

// NewMapper creates the hash mapper.
func (hopts *HashOptions) NewMapper() (Mapper, error) {
	hopts.setDefaults()
	h := &hash{}
	if err := h.init(*hopts); err != nil {
		return nil, err
	}
	return h, nil
}

func (hopts *HashOptions) setDefaults() {
	if hopts.InputLen == 0 {
		hopts.InputLen = 4
	}
	if hopts.HashBits == 0 {
		hopts.HashBits = 16
	}
}

func (hopts *HashOptions) verify() error {
	if !(2 <= hopts.InputLen && hopts.InputLen <= 8) {
		return fmt.Errorf("lz: invalid InputLen=%d; must be 2..8",
			hopts.InputLen)
	}
	maxHashBits := min(24, 8*hopts.InputLen)
	if !(0 <= hopts.HashBits && hopts.HashBits <= maxHashBits) {
		return fmt.Errorf(
			"lz: invalid HashBits=%d; must be in range 0..%d",
			hopts.HashBits, maxHashBits)
	}
	return nil
}

// GetInputLen returns the input length.
func (hopts *HashOptions) GetInputLen() int {
	return hopts.InputLen
}

var _ MapperConfigurator = (*HashOptions)(nil)

type hashJSONOptions struct {
	MapperType string
	InputLen   int `json:",omitzero"`
	HashBits   int `json:",omitzero"`
}

// MarshalJSON generates the JSON representation of HashOptions by adding the
// Mapper field and set it to "hash".
func (hopts *HashOptions) MarshalJSON() ([]byte, error) {
	jopts := hashJSONOptions{
		MapperType: "hash",
		InputLen:   hopts.InputLen,
		HashBits:   hopts.HashBits,
	}
	return json.Marshal(jopts)
}
