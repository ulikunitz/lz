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

type HashConfig struct {
	InputLen int
	HashBits int
}

func (cfg *HashConfig) ApplyDefaults() {
	if cfg.InputLen == 0 {
		cfg.InputLen = 3
	}
	if cfg.HashBits == 0 {
		cfg.HashBits = 18
	}
}

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

func (cfg *HashConfig) NewMatchFinder() (mf MatchFinder, err error) {
	cfg.ApplyDefaults()
	if err = cfg.Verify(); err != nil {
		return nil, err
	}
	h := new(hash)
	h.init(cfg.InputLen, cfg.HashBits)
	return h, nil
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

func (h *hash) Reset(pdata *[]byte) {
	h.reset()
}

func (h *hash) Adapt(delta uint32) {
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

func (h *hash) Add(pos uint32, x uint64) {
	x &= h.mask
	e := &h.table[hashValue(x, h.shift)]
	e.pos = pos
	e.value = uint32(x)
}

func (h *hash) AppendMatchesAndAdd(matches []uint32, pos uint32, x uint64) []uint32 {
	x &= h.mask
	y := uint32(x)
	e := &h.table[hashValue(x, h.shift)]
	if e.value == y {
		matches = append(matches, e.pos)
	}
	e.pos = pos
	e.value = y
	return matches
}

// prime is used for hashing
const prime = 9920624304325388887

func hashValue(x uint64, shift uint) uint32 {
	return uint32((x * prime) >> shift)
}
