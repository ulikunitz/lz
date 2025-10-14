package nlz

import "fmt"

type Matcher interface {
	Reset(data []byte) error
	Prune(n int) int
	Buf() *Buffer
	Skip(n int) (skipped int, err error)
	AppendEdges(q []Seq, n int) []Seq
}

type GreedyParserOptions struct {
	BlockSize int
}

func (opts *GreedyParserOptions) SetDefaults() {
	if opts.BlockSize <= 0 {
		opts.BlockSize = 128 << 10
	}
}

func (opts *GreedyParserOptions) Verify() error {
	if opts.BlockSize <= 0 {
		return fmt.Errorf("lz: BlockSize=%d; must be > 0",
			opts.BlockSize)
	}
	return nil
}

type GreedyParser struct {
	Matcher

	GreedyParserOptions
}

func NewGreedyParser(m Matcher, opts *GreedyParserOptions) (p *GreedyParser, err error) {
	if opts == nil {
		opts = &GreedyParserOptions{}
	}
	opts.SetDefaults()
	if err = opts.Verify(); err != nil {
		return nil, err
	}

	p = &GreedyParser{
		Matcher:             m,
		GreedyParserOptions: *opts,
	}

	return p, nil
}

func (p *GreedyParser) Parse(blk *Block, n int, flags ParserFlags) (parsed int, err error) {
	blk.Sequences = blk.Sequences[:0]
	blk.Literals = blk.Literals[:0]

	if n <= 0 {
		return 0, fmt.Errorf("lz: n=%d; must be > 0", n)
	}
	buf := p.Buf()
	n = min(n, p.BlockSize, len(buf.Data)-buf.W)
	if n == 0 {
		return 0, ErrEndOfBuffer
	}

	if blk == nil {
		return p.Matcher.Skip(n)
	}

	iLit := 0
	q := make([]Seq, 0, 128)
	b := buf.W + n
	for {
		k := b - buf.W
		if k <= 0 {
			break
		}
		q = p.AppendEdges(q[:0], k)
		if len(q) == 0 {
			panic("nlz: no edges returned by matcher")
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
				"nlz: unexpected error from Skip: %w", err))
		}
	}

	if flags&NoTrailingLiterals != 0 {
		n -= len(blk.Literals) - iLit
		blk.Literals = blk.Literals[:iLit]
	}
	return n, err
}
