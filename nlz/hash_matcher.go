package nlz

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

	BufferSize   int
	WindowSize   int
	MinMatchSize int
	MaxMatchSize int
}

// SetDefaults sets the default values for the hash options.
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
	if opt.BufferSize == 0 {
		opt.BufferSize = 64
	}
	if opt.MinMatchSize == 0 {
		opt.MinMatchSize = 2
	}
	if opt.MaxMatchSize == 0 {
		opt.MaxMatchSize = 273
	}
}

// Verify checks that the options are valid.
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
		return fmt.Errorf(
			"lz: MinMatchSize=%d; must be in range [2..MaxMatchSize=%d]",
			opt.MinMatchSize, opt.MaxMatchSize)
	}
	if !(1 < opt.BufferSize && int64(opt.BufferSize) <= math.MaxUint32) {
		return fmt.Errorf("lz: BufferSize=%d; must be in range [2..%d]",
			opt.BufferSize, math.MaxUint32)
	}
	return nil
}

// HashMatcher implements matcher of a simple hash with one entry per hash
// value.
type HashMatcher struct {
	hash hash

	Buffer

	trailing int

	HashOptions
}

// NewHashMatcher creates a new HashMatcher with the given options.
func NewHashMatcher(opts *HashOptions) (m *HashMatcher, err error) {
	if opts == nil {
		opts = &HashOptions{}
	}
	opts.SetDefaults()
	if err = opts.Verify(); err != nil {
		return nil, err
	}

	m = &HashMatcher{
		HashOptions: *opts,
	}
	if err = m.Buffer.Init(m.BufferSize); err != nil {
		return nil, err
	}
	if err = m.hash.init(m.InputLen, m.HashBits); err != nil {
		return nil, err
	}

	return m, nil
}

// Buf returns the buffer used by the matcher.
func (m *HashMatcher) Buf() *Buffer {
	return &m.Buffer
}

// Reset resets the matcher to the initial state and uses the data slice into
// the buffer.
func (m *HashMatcher) Reset(data []byte) error {
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
func (m *HashMatcher) Prune(n int) int {
	n = m.Buffer.Prune(n)
	m.hash.shiftOffsets(uint32(n))
	return n
}

// ErrEndOfBuffer is returned at the end of the buffer.
var ErrEndOfBuffer = errors.New("nlz: end of buffer")

// Skip skips n bytes in the buffer and updates the hash table.
func (m *HashMatcher) Skip(n int) (skipped int, err error) {
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
func (m *HashMatcher) AppendEdges(q []Seq, n int) []Seq {
	i := m.W
	n = min(n, m.MaxMatchSize, len(m.Data)-i)
	if n <= 0 {
		return q
	}

	b := len(m.Data) - m.InputLen + 1
	p := m.Data[:i+n]
	y := _getLE64(p[i : i+8])
	q = append(q, Seq{LitLen: 1, Aux: uint32(y) & 0xff})
	if i >= b || n < m.MinMatchSize {
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
	if k < m.MinMatchSize {
		return q
	}
	if k == 8 {
		k += lcp(p[i+8:], p[j+8:])
	}
	q = append(q, Seq{Offset: uint32(o), MatchLen: uint32(k)})
	return q
}
