package lz

import (
	"encoding/json"
	"errors"
	"math/bits"
)

// genericMatcher implements a matcher using the provided mapper^.
type genericMatcher struct {
	mapper Mapper

	Buffer

	q        []Seq
	trailing int

	windowSize int
	GenericMatcherOptions
}

// Options returns a copy of the stored options.
func (m *genericMatcher) Options() MatcherConfigurator {
	opts := m.GenericMatcherOptions
	return &opts
}

// Buf returns the buffer used by the matcher.
func (m *genericMatcher) Buf() *Buffer {
	return &m.Buffer
}

// Reset resets the matcher to the initial state and uses the data slice into
// the buffer.
func (m *genericMatcher) Reset(data []byte) error {
	if err := m.Buffer.Reset(data); err != nil {
		return err
	}
	m.mapper.Reset()

	m.trailing = 0

	return nil
}

// WindowSize returns the window size used by the matcher.
func (m *genericMatcher) WindowSize() int {
	return m.windowSize
}

// ErrEndOfBuffer is returned at the end of the buffer.
var ErrEndOfBuffer = errors.New("lz: end of buffer")

// ErrStartOfBuffer is returned at the start of the buffer.
var ErrStartOfBuffer = errors.New("lz: start of buffer")

// Skip skips n bytes in the buffer and updates the hash table.
func (m *genericMatcher) Skip(n int) (skipped int, err error) {
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
func (m *genericMatcher) Edges(n int) []Seq {
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
	for _, e := range entries {
		k := min(bits.TrailingZeros32(e.v^uint32(v))>>3, n)
		if k < m.MinMatchLen {
			continue
		}
		j := int(e.i)
		o := i - j
		if !(0 < o && o <= m.windowSize) {
			continue
		}
		if k == 4 {
			k = 4 + lcp(p[j+4:], p[i+4:])
		}
		q = append(q, Seq{Offset: uint32(o), MatchLen: uint32(k)})
	}
	m.q = q
	return q
}

// check whether genericMatcher implements Matcher.
var _ Matcher = (*genericMatcher)(nil)

// GenericMatcherOptions provide the options for a generic matcher.
type GenericMatcherOptions struct {
	MinMatchLen int
	MaxMatchLen int

	MapperOptions MapperConfigurator
}

func (opts *GenericMatcherOptions) setDefaults() {
	if opts.MinMatchLen == 0 {
		opts.MinMatchLen = 3
	}
	if opts.MaxMatchLen == 0 {
		opts.MaxMatchLen = 273
	}
	if opts.MapperOptions == nil {
		var o HashOptions
		o.setDefaults()
		opts.MapperOptions = &o
	}
}

func (opts *GenericMatcherOptions) verify() error {
	if !(1 < opts.MinMatchLen && opts.MinMatchLen <= opts.MaxMatchLen) {
		return errors.New("lz: matcher min/max match length invalid")
	}
	if !(opts.MinMatchLen <= 4) {
		return errors.New("lz: matcher MinMatchLen must be at most 4")
	}
	return nil
}

// NewMatcher creates a new generic matcher using the generic matcher options.
func (opts *GenericMatcherOptions) NewMatcher(windowSize, retentionSize, bufferSize int) (Matcher, error) {
	var o GenericMatcherOptions
	if opts != nil {
		o = *opts
	}
	var err error
	o.setDefaults()
	if err = o.verify(); err != nil {
		return nil, err
	}

	if windowSize <= 0 {
		return nil, errors.New(
			"lz: invalid window size for generic matcher")
	}

	mapper, err := o.MapperOptions.NewMapper()
	if err != nil {
		return nil, err
	}

	m := &genericMatcher{
		mapper: mapper,

		windowSize:            windowSize,
		GenericMatcherOptions: o,
	}
	if err = m.Buffer.Init(bufferSize, retentionSize, m.mapper.Shift); err != nil {
		return nil, err
	}
	return m, nil
}

var _ MatcherConfigurator = (*GenericMatcherOptions)(nil)

// MarshalJSON marshals the matcher options into JSON and adds the MatcherType
// field.
func (opts *GenericMatcherOptions) MarshalJSON() ([]byte, error) {
	jOpts := &struct {
		MatcherType string

		MinMatchLen int `json:",omitzero"`
		MaxMatchLen int `json:",omitzero"`

		MapperOptions MapperConfigurator `json:",omitzero"`
	}{
		MatcherType:   "generic",
		MinMatchLen:   opts.MinMatchLen,
		MaxMatchLen:   opts.MaxMatchLen,
		MapperOptions: opts.MapperOptions,
	}
	return json.Marshal(jOpts)
}

func (opts *GenericMatcherOptions) UnmarshalJSON(data []byte) error {
	jOpts := &struct {
		MatcherType string

		MinMatchLen int `json:",omitzero"`
		MaxMatchLen int `json:",omitzero"`

		MapperOptions json.RawMessage `json:",omitzero"`
	}{}
	var err error
	if err = json.Unmarshal(data, jOpts); err != nil {
		return err
	}
	if jOpts.MatcherType != "generic" {
		return errors.New(
			"lz: invalid matcher type for generic matcher options")
	}
	opts.MinMatchLen = jOpts.MinMatchLen
	opts.MaxMatchLen = jOpts.MaxMatchLen

	if len(jOpts.MapperOptions) > 0 {
		opts.MapperOptions, err = UnmarshalJSONMapperOptions(
			jOpts.MapperOptions)
		if err != nil {
			return err
		}
	}
	return nil
}
