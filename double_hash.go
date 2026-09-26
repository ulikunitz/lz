package lz

import (
	"fmt"
	"regexp"
	"strconv"
	"sync"
)

// doubleHash stores two independent hash tables for a value lookup.
type doubleHash struct {
	h1 hash
	h2 hash
}

// String returns the canonical name for the hash configuration.
func (d *doubleHash) String() string {
	return fmt.Sprintf(
		"doubleHash_%d:%d_%d:%d",
		d.h1.inputLen, d.h1.hashBits,
		d.h2.inputLen, d.h2.hashBits,
	)
}

// doubleHashParams contains the configuration for both hash stages.
type doubleHashParams struct {
	h1InputLen int
	h1HashBits int
	h2InputLen int
	h2HashBits int
}

// verify checks that both hash stages are configured within the supported bounds.
func (p *doubleHashParams) verify() error {
	if !(2 <= p.h1InputLen && p.h1InputLen <= 8) {
		return fmt.Errorf("lz: h1 input length must be between 2 and 8")
	}
	maxHashBits := min(24, 8*p.h1InputLen)
	if !(1 <= p.h1HashBits && p.h1HashBits <= maxHashBits) {
		return fmt.Errorf("lz: h1 hash bits must be between 1 and %d",
			maxHashBits)
	}
	if !(p.h1InputLen < p.h2InputLen && p.h2InputLen <= 8) {
		return fmt.Errorf(
			"lz: h2 input length must be greater than h1 input length %d and at most 8",
			p.h1InputLen)
	}
	maxHashBits = min(24, 8*p.h2InputLen)
	if !(1 <= p.h2HashBits && p.h2HashBits <= maxHashBits) {
		return fmt.Errorf("lz: h2 hash bits must be between 1 and %d",
			maxHashBits)
	}
	return nil
}

var doubleHashRegexp = sync.OnceValue(func() *regexp.Regexp {
	return regexp.MustCompile(`^doubleHash_(\d+):(\d+)_(\d+):(\d+)$`)
})

// parseDoubleHashName parses the name created by String.
func parseDoubleHashName(name string) (p doubleHashParams, err error) {
	m := doubleHashRegexp().FindStringSubmatch(name)
	if m == nil {
		return doubleHashParams{}, fmt.Errorf(
			"invalid double hash name: %s", name)
	}

	p.h1InputLen, err = strconv.Atoi(m[1])
	if err != nil {
		return doubleHashParams{}, fmt.Errorf("invalid h1 input length: %s", m[1])
	}
	p.h1HashBits, err = strconv.Atoi(m[2])
	if err != nil {
		return doubleHashParams{}, fmt.Errorf("invalid h1 hash bits: %s", m[2])
	}
	p.h2InputLen, err = strconv.Atoi(m[3])
	if err != nil {
		return doubleHashParams{}, fmt.Errorf("invalid h2 input length: %s", m[3])
	}
	p.h2HashBits, err = strconv.Atoi(m[4])
	if err != nil {
		return doubleHashParams{}, fmt.Errorf("invalid h2 hash bits: %s", m[4])
	}

	if err = p.verify(); err != nil {
		return doubleHashParams{}, err
	}

	return p, nil
}

// newDoubleHash creates a configured double hash from its parameters.
func newDoubleHash(p doubleHashParams) (*doubleHash, error) {
	if err := p.verify(); err != nil {
		return nil, err
	}
	var dh doubleHash
	err := dh.h1.init(p.h1InputLen, p.h1HashBits)
	if err != nil {
		return nil, err
	}
	err = dh.h2.init(p.h2InputLen, p.h2HashBits)
	if err != nil {
		return nil, err
	}
	return &dh, nil
}

// InputLen returns the effective input length of the second hash stage.
func (d *doubleHash) InputLen() int {
	return d.h2.inputLen
}

// Reset clears both underlying hash tables.
func (d *doubleHash) Reset() {
	d.h1.Reset()
	d.h2.Reset()
}

// Shift moves the hash window by delta bytes for both stages.
func (d *doubleHash) Shift(delta int) {
	d.h1.Shift(delta)
	d.h2.Shift(delta)
}

// Put inserts entries from the window into both hash tables.
func (d *doubleHash) Put(p []byte, a, w int) int {
	b := min(w, max(len(p)-max(d.h2.inputLen, 4)+1, 0))
	_p := p[:b+7]
	for i := a; i < b; i++ {
		v := _getLE64(_p[i:])
		e := Entry{i: uint32(i), v: uint32(v)}
		d.h1.table[hashValue(v&d.h1.mask, d.h1.shift)] = e
		d.h2.table[hashValue(v&d.h2.mask, d.h2.shift)] = e
	}
	return w - b
}

// AppendEntries appends all candidate entries for the provided hash value v
// from both hash tables to the provided slice and returns the updated slice.
func (d *doubleHash) AppendEntries(entries []Entry, v uint64) []Entry {
	i := hashValue(v&d.h1.mask, d.h1.shift)
	e := d.h1.table[i]
	if e.v&uint32(d.h1.mask) == uint32(v) && e != (Entry{}) {
		entries = append(entries, e)
	}

	i = hashValue(v&d.h2.mask, d.h2.shift)
	e = d.h2.table[i]
	if e.v&uint32(d.h2.mask) == uint32(v) && e != (Entry{}) {
		entries = append(entries, e)
	}
	return entries
}
