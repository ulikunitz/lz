package lz

import (
	"encoding/json"
	"fmt"
)

// GreedyParserOptions defines the configuration options for a greedy parser.
type GreedyParserOptions struct {
	MatcherOptions MatcherConfigurator
}

// NewParser creates a new greedy parser using the greedy parser options.
func (gpo GreedyParserOptions) NewParser() (Parser, error) {
	matcher, err := gpo.MatcherOptions.NewMatcher()
	if err != nil {
		return nil, fmt.Errorf(
			"lz: cannot create matcher for greedy parser: %w", err)
	}
	return &greedyParser{
		Matcher: matcher,
		options: gpo,
	}, nil
}

// greedyParser implements a greedy LZ parser.
type greedyParser struct {
	Matcher

	options GreedyParserOptions
}

// Buf returns the underlying buffer of the parser.
func (p *greedyParser) Options() Configurator {
	opts := p.options
	return &opts
}

// TODO: remove debug code or at least disable it by default.
const debugGreedyParser = true

// Parse parses up to n bytes from the underlying byte stream and appends the
// resulting sequences and literals to blk. If blk is nil, the parser will skip
// n bytes in the input stream. The number of bytes parsed or skipped is
// returned. If no more data is available, ErrEndOfBuffer is returned.
//
// If the NoTrailingLiterals flag is set, the parser will not include
// trailing literals in the block. This can be used to parse a stream in fixed
// size blocks without overlapping literals.
func (p *greedyParser) Parse(blk *Block, n int, flags ParserFlags) (parsed int, err error) {
	buf := p.Buf()
	var w int
	if debugGreedyParser {
		w = buf.W
	}
	n = min(n, buf.Parsable())
	if n == 0 {
		return 0, ErrEndOfBuffer
	}

	if blk == nil {
		return p.Matcher.Skip(n)
	}
	blk.Sequences = blk.Sequences[:0]
	blk.Literals = blk.Literals[:0]

	iLit := 0
	for parsed < n {
		q := p.Edges(n)
		if len(q) == 0 {
			panic("lz: no edges returned by matcher")
		}
		seq := q[0]
		seqLen := seq.Len()
		for _, s := range q[1:] {
			if k := s.Len(); k > seqLen {
				seq = s
				seqLen = k
			}
		}

		if seq.LitLen > 0 {
			blk.Literals = append(blk.Literals, byte(seq.Aux))
		} else {
			seq.LitLen = uint32(len(blk.Literals) - iLit)
			iLit = len(blk.Literals)
			blk.Sequences = append(blk.Sequences, seq)
		}

		_, err = p.Matcher.Skip(int(seqLen))
		if err != nil {
			panic(fmt.Errorf(
				"lz: unexpected error from Skip: %w", err))
		}

		parsed += int(seqLen)
	}

	if flags&NoTrailingLiterals != 0 {
		l := len(blk.Literals) - iLit
		_, err := p.Matcher.Skip(-l)
		if err != nil {
			panic(err)
		}
		parsed -= l
		blk.Literals = blk.Literals[:iLit]
	}

	if debugGreedyParser {
		nBuf := buf.W - w
		if nBuf != parsed {
			return n, fmt.Errorf(
				"lz: greedyParser.Parse: nBuf=%d != parsed=%d",
				nBuf, n)
		}
		nBlk, err := blk.LenCheck()
		if err != nil {
			return parsed, err
		}
		if nBlk != int64(parsed) {
			return parsed, fmt.Errorf(
				"lz: greedyParser.Parse: nBlk=%d != n=%d",
				nBlk, n)
		}
	}

	return parsed, err
}

// MarshalJSON provides a custom JSON marshaller that adds a type field to the
// JSON structure.
func (gpo *GreedyParserOptions) MarshalJSON() ([]byte, error) {
	jOpts := &struct {
		Type           string
		MatcherOptions MatcherConfigurator `json:",omitzero"`
	}{
		Type:           "greedy",
		MatcherOptions: gpo.MatcherOptions,
	}
	return json.Marshal(jOpts)
}

// UnmarshalJSON provides a custom JSON unmarshaler that parses the type
// field from the JSON structure.
func (gpo *GreedyParserOptions) UnmarshalJSON(data []byte) error {
	jOpts := &struct {
		Type           string
		MatcherOptions json.RawMessage `json:",omitzero"`
	}{}
	var err error
	if err = json.Unmarshal(data, jOpts); err != nil {
		return err
	}
	if jOpts.Type != "greedy" {
		return fmt.Errorf(
			"lz: invalid parser type for greedy parser options: %q",
			jOpts.Type)
	}
	if len(jOpts.MatcherOptions) > 0 {
		gpo.MatcherOptions, err = UnmarshalJSONMatcherOptions(
			jOpts.MatcherOptions)
		if err != nil {
			return err
		}
	}
	return nil
}

// test that the implementations satisfy the interfaces.
var (
	_ Parser       = (*greedyParser)(nil)
	_ Configurator = (*GreedyParserOptions)(nil)
)
