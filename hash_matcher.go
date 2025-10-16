package lz

import (
	"errors"
	"fmt"
	"math"
	"math/bits"
)

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

var _ = (MatcherOptions)(&HashOptions{})

// hashMatcher implements matcher of a simple hash with one entry per hash
// value.
type hashMatcher struct {
	hash hash

	Buffer

	trailing int

	HashOptions
}

// NewMatcher creates a new HashMatcher with the given options.
func (opts *HashOptions) NewMatcher() (Matcher, error) {
	if opts == nil {
		opts = &HashOptions{}
	}
	opts.setDefaults()

	var err error
	if err = opts.verify(); err != nil {
		return nil, err
	}

	hm := &hashMatcher{
		HashOptions: *opts,
	}
	if err = hm.Buffer.Init(hm.BufferSize); err != nil {
		return nil, err
	}
	if err = hm.hash.init(hm.InputLen, hm.HashBits); err != nil {
		return nil, err
	}

	return hm, nil
}

// Buf returns the buffer used by the matcher.
func (m *hashMatcher) Buf() *Buffer {
	return &m.Buffer
}

// Reset resets the matcher to the initial state and uses the data slice into
// the buffer.
func (m *hashMatcher) Reset(data []byte) error {
	if err := m.Buffer.Reset(data); err != nil {
		return err
	}
	m.hash.reset()

	m.trailing = 0

	return nil
}

// Prune removes n bytes from the beginning of the buffer and updates the hash
// table accordingly. It returns the actual number of bytes removed which can be
// less than n if n is greater than the buffer that can be pruned.
func (m *hashMatcher) Prune(n int) int {
	if n == 0 {
		n = max(m.W-m.WindowSize, 3*(m.Buffer.Size/4), 0)
	}
	n = m.Buffer.Prune(n)
	m.hash.shiftOffsets(uint32(n))
	return n
}

// ErrEndOfBuffer is returned at the end of the buffer.
var ErrEndOfBuffer = errors.New("lz: end of buffer")

// ErrStartOfBuffer is returned at the start of the buffer.
var ErrStartOfBuffer = errors.New("lz: start of buffer")

// Skip skips n bytes in the buffer and updates the hash table.
func (m *hashMatcher) Skip(n int) (skipped int, err error) {
	if n < 0 {
		if n < -m.W {
			n = -m.W
			err = ErrStartOfBuffer
		}
		m.W += n
		m.trailing = max(m.trailing+n, 0)
		return n, err
	}

	if k := len(m.Data) - m.W; k < n {
		n = k
		err = ErrEndOfBuffer
	}

	a := max(m.W-m.trailing, 0)
	m.W += n
	b := min(m.W, len(m.Data)-m.InputLen+1)
	m.trailing = m.W - b
	if a < b {
		p := m.Data[:b+7]
		for i := a; i < b; i++ {
			x := _getLE64(p[i:]) & m.hash.mask
			h := hashValue(x, m.hash.shift)
			m.hash.table[h] = hashEntry{
				pos:   uint32(i),
				value: uint32(x),
			}
		}
	}

	return n, err
}

// AppendEdges appends the literal and the matches found at the current
// position. This function returns the literal and at most one match.
//
// n limits the maximum length for a match and can be used to restrict the
// matches to the end of the block to parse.
func (m *hashMatcher) AppendEdges(q []Seq, n int) []Seq {
	i := m.W
	n = min(n, m.MaxMatchLen, len(m.Data)-i)
	if n <= 0 {
		return q
	}

	b := len(m.Data) - m.InputLen + 1
	p := m.Data[:i+n]
	y := _getLE64(p[i : i+8])
	q = append(q, Seq{LitLen: 1, Aux: uint32(y) & 0xff})
	if i >= b || n < m.MinMatchLen {
		return q
	}

	x := y & m.hash.mask
	h := hashValue(x, m.hash.shift)
	entry := &m.hash.table[h]
	if entry.value != uint32(x) {
		return q
	}

	j := int(entry.pos)
	o := i - j
	if !(0 < o && o <= m.WindowSize) {
		return q
	}
	k := min(bits.TrailingZeros64(_getLE64(p[j:])^y)>>3, len(p)-i)
	if k < m.MinMatchLen {
		return q
	}
	if k == 8 {
		k += lcp(p[i+8:], p[j+8:])
	}
	q = append(q, Seq{Offset: uint32(o), MatchLen: uint32(k)})
	return q
}

// ensure that the HashMatcher implements the Matcher interface.
var _ Matcher = (*hashMatcher)(nil)
