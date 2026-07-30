package lz

import (
	"errors"
	"fmt"
	"math/bits"
)

// genericParser is a generic implementation of the Parser interface. It uses a
// PathFinder to find matches in the input data and a Mapper to map the input
// data to a sequence of bytes. The genericParser maintains a buffer of input
// data.
type genericParser struct {
	PathFinder

	mapper Mapper

	Buffer

	q        []Seq
	trailing int

	ParserConfig
}

// newGenericParser creates a new genericParser with the given configuration.
// Note that no copy of the configuration will be made.
func newGenericParser(cfg *ParserConfig) (*genericParser, error) {
	cfg.setDefaults()
	if *cfg.WindowSize < 0 {
		return nil, fmt.Errorf("lz: invalid window size %d; must be >= 0",
			*cfg.WindowSize)
	}
	if !(2 <= cfg.MinMatchLen && cfg.MinMatchLen <= cfg.MaxMatchLen) {
		return nil, fmt.Errorf(
			"lz: invalid min match length %d; must be between 2 and max match length %d",
			cfg.MinMatchLen, cfg.MaxMatchLen)
	}

	mapper, err := NewMapper(cfg.Mapper)
	if err != nil {
		return nil, err
	}
	gp := &genericParser{
		mapper:       mapper,
		ParserConfig: *cfg,
	}
	err = gp.Buffer.Init(cfg.BufferSize, *cfg.RetentionSize, mapper.Shift)
	if err != nil {
		return nil, err
	}
	gp.PathFinder, err = NewPathFinder(cfg.PathFinder, gp)
	if err != nil {
		return nil, err
	}

	return gp, nil
}

// Config returns a deep copy of the ParserConfig value used to create the
// genericParser.
func (gp *genericParser) Config() ParserConfig {
	return gp.ParserConfig.clone()
}

// Edges return a slice of Seq values representing the edges of the graph at the
// current position in the input data. The edges are computed by the Mapper.
func (gp *genericParser) Edges(n int) []Seq {
	q := gp.q[:0]
	i := gp.W
	n = min(n, len(gp.Data)-i)
	if n <= 0 {
		return q
	}

	b := len(gp.Data) - gp.mapper.InputLen() + 1
	p := gp.Data[:i+n]
	v := _getLE64(p[i : i+8])
	q = append(q, Seq{LitLen: 1, Aux: uint32(v & 0xFF)})
	gp.q = q
	if i >= b || n < gp.MinMatchLen {
		return q
	}

	entries := gp.mapper.Get(v)
	for _, e := range entries {
		k := min(bits.TrailingZeros32(e.v^uint32(v))>>3, n)
		if k < gp.MinMatchLen {
			continue
		}
		j := int(e.i)
		o := i - j
		if !(0 < o && o <= *gp.WindowSize) {
			continue
		}
		if k == 4 {
			k = 4 + lcp(p[j+4:], p[i+4:])
		}
		q = append(q, Seq{Offset: uint32(o), MatchLen: uint32(k)})
	}
	gp.q = q
	return q
}

// ErrEndOfBuffer is returned at the end of the buffer.
var ErrEndOfBuffer = errors.New("lz: end of buffer")

// ErrStartOfBuffer is returned at the start of the buffer.
var ErrStartOfBuffer = errors.New("lz: start of buffer")

// Skip jumps over the next n bytes in the input data. The argument n can be
// negative.
func (gp *genericParser) Skip(n int) (skipped int, err error) {
	if n < 0 {
		if n < -gp.W {
			n = -gp.W
			err = ErrStartOfBuffer
		}
		gp.W += n
		gp.trailing = max(gp.trailing+n, 0)
		return n, err
	}

	if k := len(gp.Data) - gp.W; k < n {
		n = k
		err = ErrEndOfBuffer
	}

	a := max(gp.W-gp.trailing, 0)
	gp.W += n
	if a < gp.W {
		gp.trailing = gp.mapper.Put(gp.Data, a, gp.W)
	}

	return n, err
}

// Resets the parser for being reused with a new input data slice.
func (gp *genericParser) Reset(data []byte) error {
	var err error
	if err = gp.Buffer.Reset(data); err != nil {
		return err
	}
	gp.mapper.Reset()
	gp.PathFinder.Reset()
	gp.trailing = 0
	return nil
}

// verify that the genericParser implements the Parser interface.
var _ Parser = (*genericParser)(nil)
