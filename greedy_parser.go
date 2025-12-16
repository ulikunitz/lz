package lz

import "fmt"

// defaultBlockSize defines the standard block size for a Parser.
const defaultBlockSize = 1 << 17 // 128 KiB

type GreedyParserOptions struct {
	BlockSize int
	Matcher   MatcherConfigurator
}

func (gpo *GreedyParserOptions) NewParser() (Parser, error) {
	if gpo.BlockSize <= 0 {
		if gpo.BlockSize < 0 {
			return nil, fmt.Errorf(
				"lz: greedy parser block size cannot be negative: %d",
				gpo.BlockSize)
		}
		gpo.BlockSize = defaultBlockSize
	}

	matcher, err := gpo.Matcher.NewMatcher()
	if err != nil {
		return nil, fmt.Errorf(
			"lz: cannot create matcher for greedy parser: %w", err)
	}
	return &greedyParser{
		Matcher: matcher,
		options: *gpo,
	}, nil
}

type greedyParser struct {
	Matcher

	options GreedyParserOptions
}

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
func (p *greedyParser) Parse(blk *Block, flags ParserFlags) (parsed int, err error) {
	blockSize := p.options.BlockSize

	n := blockSize
	buf := p.Buf()
	w := buf.W
	n = min(n, len(buf.Data)-w)
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

	if debugGreedyParser {
		nBuf := buf.W - w
		if nBuf != n {
			return n, fmt.Errorf(
				"lz: greedyParser.Parse: nBuf=%d != n=%d",
				nBuf, n)
		}
		nBlk, err := blk.LenCheck()
		if err != nil {
			return n, err
		}
		if nBlk != int64(n) {
			return n, fmt.Errorf(
				"lz: greedyParser.Parse: nBlk=%d != n=%d",
				nBlk, n)
		}
	}

	return n, err
}

var _ Parser = (*greedyParser)(nil)
