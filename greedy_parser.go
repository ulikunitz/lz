package lz

import (
	"fmt"
)

// GreedyParserOptions contains the options for the GreedyParser. Right now only
// the default block size can be configured.
type GreedyParserOptions struct {
	BlockSize int

	MatcherOptions MatcherOptions
}

// SetWindowSize sets the window size for the parser and its matcher.
func (opts *GreedyParserOptions) SetWindowSize(size int) {
	if opts.MatcherOptions == nil {
		opts.MatcherOptions = &HashOptions{}
	}
	opts.MatcherOptions.SetWindowSize(size)
}

// SetDefaults sets the default values for the GreedyParser options.
func (opts *GreedyParserOptions) setDefaults() {
	if opts.BlockSize <= 0 {
		opts.BlockSize = 128 << 10
	}
	if opts.MatcherOptions == nil {
		opts.MatcherOptions = &HashOptions{}
	}
}

// Verify checks that the options are valid.
func (opts *GreedyParserOptions) verify() error {
	if opts.BlockSize <= 0 {
		return fmt.Errorf("lz: BlockSize=%d; must be > 0",
			opts.BlockSize)
	}
	return nil
}

// greedyParser is a simple parser that always chooses the longest match.
type greedyParser struct {
	Matcher

	GreedyParserOptions
}

// NewParser creates a new GreedyParser with the given options. If
// opts is nil, the default options are used.
func (opts *GreedyParserOptions) NewParser() (Parser, error) {
	if opts == nil {
		opts = &GreedyParserOptions{}
	}
	opts.setDefaults()
	var err error
	if err = opts.verify(); err != nil {
		return nil, err
	}

	m, err := opts.MatcherOptions.NewMatcher()
	if err != nil {
		return nil, err
	}

	gp := &greedyParser{
		Matcher:             m,
		GreedyParserOptions: *opts,
	}

	return gp, nil
}

// Parse parses up to n bytes from the underlying byte stream and appends the
// resulting sequences and literals to blk. If blk is nil, the parser will skip
// n bytes in the input stream. The number of bytes parsed or skipped is
// returned. If no more data is available, ErrEndOfBuffer is returned.
//
// If the NoTrailingLiterals flag is set, the parser will not include
// trailing literals in the block. This can be used to parse a stream in fixed
// size blocks without overlapping literals.
func (p *greedyParser) Parse(blk *Block, n int, flags ParserFlags) (parsed int, err error) {

	if n < 0 {
		return 0, fmt.Errorf("lz: n=%d; must be >= 0", n)
	}
	if n == 0 {
		n = p.BlockSize
	}

	buf := p.Buf()
	n = min(n, p.BlockSize, len(buf.Data)-buf.W)
	if n == 0 {
		return 0, ErrEndOfBuffer
	}

	if blk == nil {
		return p.Matcher.Skip(n)
	}
	blk.Sequences = blk.Sequences[:0]
	blk.Literals = blk.Literals[:0]

	iLit := 0
	b := buf.W + n
	for {
		k := b - buf.W
		if k <= 0 {
			break
		}
		q := p.Edges(k)
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
	}

	if flags&NoTrailingLiterals != 0 {
		l := len(blk.Literals) - iLit
		_, err := p.Matcher.Skip(-l)
		if err != nil {
			panic(err)
		}
		n -= l
		blk.Literals = blk.Literals[:iLit]
	}
	return n, err
}

// ensure that GreedyParser implements the Parser interface.
var _ Parser = (*greedyParser)(nil)
