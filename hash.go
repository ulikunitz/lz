package lz

import (
	"fmt"
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

// The hash implements a match finder and can be directly used in a sequencer.
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

// HashConfig provides the configuration for the hash match finder.
type HashConfig struct {
	InputLen int
	HashBits int
}

// ApplyDefaults sets the defaults of the HashConfig. The default input length
// is 3 and the hash bits are 18.
func (cfg *HashConfig) ApplyDefaults() {
	if cfg.InputLen == 0 {
		cfg.InputLen = 3
	}
	if cfg.HashBits == 0 {
		cfg.HashBits = 18
	}
}

// Verify checks the configuration parameters.
func (cfg *HashConfig) Verify() error {
	if !(2 <= cfg.InputLen && cfg.InputLen <= 8) {
		return fmt.Errorf("lz: InputLen must be in range [2,8]")
	}
	maxHashBits := 24
	if t := 8 * cfg.InputLen; t < maxHashBits {
		maxHashBits = t
	}
	if !(0 <= cfg.HashBits && cfg.HashBits <= maxHashBits) {
		return fmt.Errorf("lz: HashBits=%d; must be <= %d",
			cfg.HashBits, maxHashBits)
	}
	return nil
}

// NewMatchFinder initializes a new match finder.
func (cfg *HashConfig) NewMatchFinder() (mf MatchFinder, err error) {
	cfg.ApplyDefaults()
	if err = cfg.Verify(); err != nil {
		return nil, err
	}
	f := new(hashFinder)
	f.hash.init(cfg.InputLen, cfg.HashBits)
	return f, err
}

type hashFinder struct {
	hash
	data  []byte
	_data []byte
}

// Update informs the hash finder to data changes in the data slice. If delta is
// less than zero than complete new data is provided. If the delta is positive
// data has been moved delta bytes down in the slice. If delta is zero data has
// been added.
func (f *hashFinder) Update(p []byte, delta int) {
	switch {
	case delta < 0:
		f.hash.reset()
	case delta > 0:
		f.hash.shiftOffsets(uint32(delta))
	}
	if len(p) == 0 {
		f.hash.reset()
		f.data = f.data[:0]
		return
	}
	if len(p)+7 > cap(p) {
		if len(p)+7 > cap(f.data) {
			f.data = make([]byte, len(p), len(p)+7)
		} else {
			f.data = f.data[:len(p)]
		}
		copy(f.data, p)
	} else {
		f.data = p
	}
	f._data = f.data[:len(p)+7]
}

// ProcessSegment adds the hashes between position a and b into the hash.
func (f *hashFinder) ProcessSegment(a, b int) {
	c := len(f.data) - f.inputLen + 1
	if c < b {
		b = c
	}
	if b <= 0 {
		return
	}

	_p := f._data
	for i := a; i < b; i++ {
		x := _getLE64(_p[i:]) & f.mask
		f.table[hashValue(x, f.shift)] = hashEntry{
			pos:   uint32(i),
			value: uint32(x),
		}
	}
}

// AppendMatchOffsets extracts a single offset from the hash table and writes
// the hash for the current position into the hash.
func (f *hashFinder) AppendMatchOffsets(m []uint32, i int) []uint32 {
	x := _getLE64(f._data[i:]) & f.mask
	y := uint32(x)
	e := &f.table[hashValue(x, f.shift)]
	if e.value == y {
		m = append(m, e.pos)
	}
	e.pos = uint32(i)
	e.value = y
	return m
}
