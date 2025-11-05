package lz

import (
	"errors"
	"io"
	"math/bits"
)

// matcher is responsible to find matches or Literal bytes in the byte stream.
type matcher interface {
	Edges(n int) []Seq
	Skip(n int) (skipped int, err error)

	Prune(n int) int
	Write(p []byte) (n int, err error)
	ReadFrom(r io.Reader) (n int64, err error)

	ReadAt(p []byte, off int64) (n int, err error)
	ByteAt(off int64) (c byte, err error)

	Reset(data []byte) error
	Buf() *Buffer
}

// genericMatcher[M] implements a matcher using the provided mapper M.
type genericMatcher[M mapper] struct {
	mapper M

	Buffer

	q        []Seq
	trailing int

	MinMatchLen    int
	MaxMatchLen    int
	WindowSize     int
	NoPruning      bool
	MaintainWindow bool
}

func newMatcher[M mapper](m M, opts *ParserOptions) (*genericMatcher[M], error) {
	var err error
	if opts == nil {
		return nil, errors.New("lz: matcher options missing")
	}

	matcher := &genericMatcher[M]{
		mapper:         m,
		MinMatchLen:    opts.MinMatchLen,
		MaxMatchLen:    opts.MaxMatchLen,
		WindowSize:     opts.WindowSize,
		NoPruning:      opts.NoPruning,
		MaintainWindow: opts.MaintainWindow,
	}
	if err = matcher.Buffer.Init(opts.BufferSize); err != nil {
		return nil, err
	}
	return matcher, nil

}

// Buf returns the buffer used by the matcher.
func (m *genericMatcher[M]) Buf() *Buffer {
	return &m.Buffer
}

// Reset resets the matcher to the initial state and uses the data slice into
// the buffer.
func (m *genericMatcher[M]) Reset(data []byte) error {
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
func (m *genericMatcher[M]) Prune(n int) int {
	if m.NoPruning {
		return 0
	}
	if n == 0 {
		n = 3 * (m.Buffer.Size / 4)
	}
	if m.MaintainWindow {
		n = min(n, m.W-m.WindowSize)
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
func (m *genericMatcher[M]) Skip(n int) (skipped int, err error) {
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
func (m *genericMatcher[M]) Edges(n int) []Seq {
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

var _ matcher = (*genericMatcher[*hash])(nil)
