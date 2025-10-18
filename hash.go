package lz

import (
	"errors"
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
	b := max(len(p)-h.inputLen+1, w, 0)
	_p := p[:b+7]
	for i := a; i < b; i++ {
		v := _getLE64(_p[i:])
		h.table[hashValue(v&h.mask, h.shift)] =
			Entry{i: uint32(i), v: uint32(v)}
	}
	return w - b
}

func (h *hash) Get(v uint64) []Entry {
	i := hashValue(v&h.mask, h.shift)
	return h.table[i : i+1]
}

// HashOptions contains the options for the HashMatcher.
type HashOptions struct {
	InputLen int
	HashBits int

	BufferSize  int
	WindowSize  int
	MinMatchLen int
	MaxMatchLen int
}

// SetWindowSize sets the window size for the matcher.
func (opts *HashOptions) SetWindowSize(size int) {
	opts.WindowSize = size
}

// setDefaults sets the default values for the hash options.
func (opts *HashOptions) setDefaults() {
	if opts.InputLen == 0 {
		opts.InputLen = 4
	}
	if opts.HashBits == 0 {
		opts.HashBits = 16
	}
	if opts.WindowSize == 0 {
		opts.WindowSize = 32 << 10
	}
	if opts.BufferSize == 0 {
		opts.BufferSize = opts.WindowSize
	}
	if opts.MinMatchLen == 0 {
		opts.MinMatchLen = 2
	}
	if opts.MaxMatchLen == 0 {
		opts.MaxMatchLen = 273
	}
}

// verify checks that the options are valid.
func (opts *HashOptions) verify() error {
	if !(2 <= opts.InputLen && opts.InputLen <= 8) {
		return fmt.Errorf("lz: InputLen must be in range [2,8]")
	}
	maxHashBits := 24
	if t := 8 * opts.InputLen; t < maxHashBits {
		maxHashBits = t
	}
	if !(0 <= opts.HashBits && opts.HashBits <= maxHashBits) {
		return fmt.Errorf("lz: hashBits=%d; must be <= %d",
			opts.HashBits, maxHashBits)
	}
	if !(0 <= opts.WindowSize && int64(opts.WindowSize) <= math.MaxUint32) {
		return fmt.Errorf("lz: WindowSize=%d; must be in range [0..%d]",
			opts.WindowSize, math.MaxUint32)
	}
	if !(2 <= opts.MinMatchLen && opts.MinMatchLen <= opts.MaxMatchLen) {
		return fmt.Errorf(
			"lz: MinMatchLen=%d; must be in range [2..MaxMatchLen=%d]",
			opts.MinMatchLen, opts.MaxMatchLen)
	}
	if !(1 < opts.BufferSize && int64(opts.BufferSize) <= math.MaxUint32) {
		return fmt.Errorf("lz: BufferSize=%d; must be in range [2..%d]",
			opts.BufferSize, math.MaxUint32)
	}
	return nil
}

func (opts *HashOptions) NewMatcher() (Matcher, error) {
	var err error

	opts.setDefaults()
	if err = opts.verify(); err != nil {
		return nil, err
	}

	h := new(hash)
	if err = h.init(opts.InputLen, opts.HashBits); err != nil {
		return nil, err
	}

	matcherOpts := &matcherOptions{
		WindowSize:  opts.WindowSize,
		BufferSize:  opts.BufferSize,
		MinMatchLen: opts.MinMatchLen,
		MaxMatchLen: opts.MaxMatchLen,
	}

	return newMatcher(h, matcherOpts)
}

var _ = (MatcherOptions)(&HashOptions{})
