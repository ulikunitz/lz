// SPDX-FileCopyrightText: © 2021 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

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

// The hash implements a match finder and can be directly used in a parser.
type hash struct {
	table    []hashEntry
	mask     uint64
	shift    uint
	inputLen int
}

// HashConfig describes the configuration for a single Hash.
type HashConfig struct {
	InputLen int
	HashBits int
}

// init initializes the hash structure.
func (h *hash) init(cfg HashConfig) error {
	if !(2 <= cfg.InputLen && cfg.InputLen <= 8) {
		return fmt.Errorf("lz: inputLen must be in range [2,8]")
	}
	maxHashBits := 24
	if t := 8 * cfg.InputLen; t < maxHashBits {
		maxHashBits = t
	}
	if !(0 <= cfg.HashBits && cfg.HashBits <= maxHashBits) {
		return fmt.Errorf("lz: hashBits=%d; must be <= %d",
			cfg.HashBits, maxHashBits)
	}

	n := 1 << cfg.HashBits
	if n <= cap(h.table) {
		h.table = h.table[:n]
		for i := range h.table {
			h.table[i] = hashEntry{}
		}
	} else {
		h.table = make([]hashEntry, n)
	}

	h.mask = 1<<(uint(cfg.InputLen)*8) - 1
	h.shift = 64 - uint(cfg.HashBits)
	h.inputLen = cfg.InputLen

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

// setDefaults sets the defaults of the HashConfig. The default input length
// is 3 and the hash bits are 18.
func (cfg *HashConfig) setDefaults() {
	if cfg.InputLen == 0 {
		cfg.InputLen = 3
	}
	if cfg.HashBits == 0 {
		cfg.HashBits = 18
	}
}

// verify checks the configuration parameters.
func (cfg *HashConfig) verify() error {
	if !(2 <= cfg.InputLen && cfg.InputLen <= 8) {
		return fmt.Errorf("lz: InputLen must be in range [2..8]")
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

type hashDictionary struct {
	Buffer
	hash
}

func (f *hashDictionary) init(cfg HashConfig, bcfg BufConfig) error {
	var err error
	if err = f.Buffer.Init(bcfg); err != nil {
		return err
	}
	cfg.setDefaults()
	if err = cfg.verify(); err != nil {
		return err
	}
	err = f.hash.init(cfg)
	return err
}

func (f *hashDictionary) Reset(data []byte) error {
	var err error
	if err = f.Buffer.Reset(data); err != nil {
		return err
	}
	f.hash.reset()
	return nil
}

func (f *hashDictionary) Shrink() int {
	delta := f.Buffer.Shrink()
	if delta > 0 {
		f.hash.shiftOffsets(uint32(delta))
	}
	return delta
}

// ProcessSegment adds the hashes between position a and b into the hash.
func (f *hashDictionary) processSegment(a, b int) {
	if a < 0 {
		a = 0
	}
	c := len(f.Data) - f.inputLen + 1
	if c < b {
		b = c
	}
	if b <= 0 {
		return
	}

	_p := f.Data[:b+7]
	for i := a; i < b; i++ {
		x := _getLE64(_p[i:]) & f.mask
		f.table[hashValue(x, f.shift)] = hashEntry{
			pos:   uint32(i),
			value: uint32(x),
		}
	}
}

type doubleHashDictionary struct {
	Buffer
	h1 hash
	h2 hash
}

func (f *doubleHashDictionary) init(h1cfg, h2cfg HashConfig, bcfg BufConfig) error {
	var err error
	if err = f.Buffer.Init(bcfg); err != nil {
		return err
	}
	h1cfg.setDefaults()
	if err = h1cfg.verify(); err != nil {
		return err
	}
	h2cfg.setDefaults()
	if err = h2cfg.verify(); err != nil {
		return err
	}
	if h1cfg.InputLen < h2cfg.InputLen {
		return fmt.Errorf("lz: h1 must have shorter InputLen than h2")
	}
	if err = f.h1.init(h1cfg); err != nil {
		return err
	}
	err = f.h2.init(h2cfg)
	return err
}

func (f *doubleHashDictionary) Shrink() int {
	delta := f.Buffer.Shrink()
	if delta > 0 {
		f.h1.shiftOffsets(uint32(delta))
		f.h2.shiftOffsets(uint32(delta))
	}
	return delta
}

// processSegment adds the hashes between position a and b into the hash.
func (f *doubleHashDictionary) processSegment(a, b int) {
	if a < 0 {
		a = 0
	}
	h1, h2 := &f.h1, &f.h2

	b1, c1 := b, len(f.Data)-h1.inputLen+1
	if c1 < b1 {
		b1 = c1
	}
	if b1 < 0 {
		b1 = 0
	}
	b2, c2 := b, len(f.Data)-h2.inputLen+1
	if c2 < b2 {
		b2 = c2
	}
	if b2 < 0 {
		b2 = 0
	}

	_p := f.Data[:b1+7]
	for i := a; i < b2; i++ {
		x := _getLE64(_p[i:])
		e := hashEntry{
			pos:   uint32(i),
			value: uint32(x),
		}
		h1.table[hashValue(x&h1.mask, h1.shift)] = e
		h2.table[hashValue(x&h2.mask, h2.shift)] = e
	}
	for i := b2; i < b1; i++ {
		x := _getLE64(_p[i:])
		h1.table[hashValue(x&h1.mask, h1.shift)] = hashEntry{
			pos:   uint32(i),
			value: uint32(x),
		}
	}
}
