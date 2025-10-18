package lz

import (
	"errors"
	"fmt"
	"math"
	"math/bits"
)

// matcher[M] implements matcher of a simple hash with one entry per hash
// value.
type matcher[M Mapper] struct {
	mapper M

	Buffer

	q        []Seq
	trailing int

	matcherOptions
}

type matcherOptions struct {
	WindowSize  int
	BufferSize  int
	MinMatchLen int
	MaxMatchLen int
}

func (opts *matcherOptions) setDefaults() {
	if opts.WindowSize == 0 {
		opts.WindowSize = 32 << 10
	}
	if opts.BufferSize == 0 {
		opts.BufferSize = 8 << 20
	}
	if opts.MinMatchLen == 0 {
		opts.MinMatchLen = 2
	}
	if opts.MaxMatchLen == 0 {
		opts.MaxMatchLen = 273
	}
}

func (opts *matcherOptions) verify() error {
	if !(0 <= opts.WindowSize && int64(opts.WindowSize) <= math.MaxUint32) {
		return fmt.Errorf("lz: WindowSize=%d; must be in range [0..%d]",
			opts.WindowSize, math.MaxUint32)
	}
	if !(1 < opts.BufferSize && int64(opts.BufferSize) <= math.MaxUint32) {
		return fmt.Errorf("lz: BufferSize=%d; must be in range [2..%d]",
			opts.BufferSize, math.MaxUint32)
	}
	if !(0 < opts.MinMatchLen && int64(opts.MinMatchLen) <= math.MaxUint32) {
		return fmt.Errorf("lz: MinMatchLen=%d; must be in range [1..%d]",
			opts.MinMatchLen, math.MaxUint32)
	}
	if !(0 < opts.MaxMatchLen && int64(opts.MaxMatchLen) <= math.MaxUint32) {
		return fmt.Errorf("lz: MaxMatchLen=%d; must be in range [1..%d]",
			opts.MaxMatchLen, math.MaxUint32)
	}
	if !(opts.MinMatchLen <= opts.MaxMatchLen) {
		return fmt.Errorf(
			"lz: MinMatchLen=%d; must be <= MaxMatchLen=%d",
			opts.MinMatchLen, opts.MaxMatchLen)
	}
	return nil
}

func newMatcher[M Mapper](m M, opts *matcherOptions) (*matcher[M], error) {
	var err error
	if opts == nil {
		opts = &matcherOptions{}
	}
	opts.setDefaults()
	if err = opts.verify(); err != nil {
		return nil, err
	}

	matcher := &matcher[M]{
		mapper:         m,
		matcherOptions: *opts,
	}
	if err = matcher.Buffer.Init(opts.BufferSize); err != nil {
		return nil, err
	}
	return matcher, nil

}

// Buf returns the buffer used by the matcher.
func (m *matcher[M]) Buf() *Buffer {
	return &m.Buffer
}

// Reset resets the matcher to the initial state and uses the data slice into
// the buffer.
func (m *matcher[M]) Reset(data []byte) error {
	if err := m.Buffer.Reset(data); err != nil {
		return err
	}
	m.mapper.Reset()

	m.trailing = 0

	return nil
}

// Prune removes n bytes from the beginning of the buffer and updates the hash
// table accordingly. It returns the actual number of bytes removed which can be
// less than n if n is greater than the buffer that can be pruned.
func (m *matcher[M]) Prune(n int) int {
	if n == 0 {
		n = max(m.W-m.WindowSize, 3*(m.Buffer.Size/4), 0)
	}
	n = m.Buffer.Prune(n)
	m.mapper.Shift(n)
	return n
}

// ErrEndOfBuffer is returned at the end of the buffer.
var ErrEndOfBuffer = errors.New("lz: end of buffer")

// ErrStartOfBuffer is returned at the start of the buffer.
var ErrStartOfBuffer = errors.New("lz: start of buffer")

// Skip skips n bytes in the buffer and updates the hash table.
func (m *matcher[M]) Skip(n int) (skipped int, err error) {
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
	if a < m.W {
		m.trailing = m.mapper.Put(a, m.W, m.Data)
	}

	return n, err
}

// Edges appends the literal and the matches found at the current
// position. This function returns the literal and at most one match.
//
// n limits the maximum length for a match and can be used to restrict the
// matches to the end of the block to parse.
func (m *matcher[M]) Edges(n int) []Seq {
	q := m.q[:0]
	i := m.W
	n = min(n, m.MaxMatchLen, len(m.Data)-i)
	if n <= 0 {
		return q
	}

	b := len(m.Data) - m.mapper.InputLen() + 1
	p := m.Data[:i+n]
	v := _getLE64(p[i : i+8])
	q = append(q, Seq{LitLen: 1, Aux: uint32(v) & 0xff})
	m.q = q
	if i >= b || n < m.MinMatchLen {
		return q
	}

	entries := m.mapper.Get(v)
	kMax := len(p) - i
	for _, e := range entries {
		k := min(bits.TrailingZeros32(uint32(v)^e.v)>>3, kMax)
		if k < m.MinMatchLen {
			continue
		}
		j := int(e.i)
		o := i - j
		if !(0 < o && o <= m.WindowSize) {
			continue
		}
		if k == 4 {
			k += lcp(p[i+4:], p[j+4:])
		}
		q = append(q, Seq{Offset: uint32(o), MatchLen: uint32(k)})
	}
	m.q = q
	return q
}

// ensure that the matcher[M] implements the Matcher interface.
var _ Matcher = (*matcher[*hash])(nil)
