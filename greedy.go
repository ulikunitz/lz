package lz

import "fmt"

type GreedyPathFinder struct {
	matcher Matcher
}

func (f *GreedyPathFinder) Parse(blk *Block, n int, flags ParserFlags) (parsed int, err error) {
	if n <= 0 {
		return 0, fmt.Errorf("lz: length %d <= 0; want n > 0", n)
	}

	p := f.matcher
	n = min(n, p.Parsable())
	if n == 0 {
		return 0, ErrEndOfBuffer
	}

	if blk == nil {
		return p.Skip(n)
	}

	blk.Literals = blk.Literals[:0]
	blk.Sequences = blk.Sequences[:0]

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

		_, err = p.Skip(int(seqLen))
		if err != nil {
			panic(fmt.Errorf(
				"lz: unexpected error from Skip: %w", err))
		}

		parsed += int(seqLen)
	}

	if flags&NoTrailingLiterals != 0 {
		l := len(blk.Literals) - iLit
		_, err := p.Skip(-l)
		if err != nil {
			panic(err)
		}
		parsed -= l
		blk.Literals = blk.Literals[:iLit]
	}

	return parsed, nil
}

func (f *GreedyPathFinder) Reset() {}
