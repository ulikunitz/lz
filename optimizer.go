package lz

// We have to rethink this again.
//
// We keep it simply we maintain a descending list of matches (m, o) for each
// position. So m_i > m_j and o_i > o_j with i < j. Literals are implicit.

type match struct {
	m, o uint32
}

func mergeMatches(out, a, b []match) []match {
	for {
		if len(a) == 0 {
			return append(out, b...)
		}
		if len(b) == 0 {
			return append(out, a...)
		}
		x, y := a[0], b[0]
		for {
			switch {
			case x.m > y.m:
				out = append(out, x)
				a = a[1:]
				if len(a) == 0 {
					return append(out, b...)
				}
				x = a[0]
				continue
			case x.m < y.m:
				out = append(out, y)
				b = b[1:]
				if len(b) == 0 {
					return append(out, a...)
				}
				y = a[0]
				continue
			case x.o < y.o:
				out = append(out, x)
			default:
				out = append(out, y)
			}
			a, b = a[1:], b[1:]
			break
		}
	}
}

type matchMap [][]match

type optrec struct {
	c        uint32
	off      uint32
	matchLen uint32
}

type optimizer struct {
	blk         *Block
	p           []byte
	m           [][]match
	cost        costFn
	minMatchLen uint32
	flags       int
	a           []optrec
}

type costFn func(offset, matchLen uint32) uint32

func (o *optimizer) sequence() int {
	/*
		n := len(o.p)

		// reconstruct seq and handle literals
		o.m = o.m[:0]
		i := n
		for i > 0 {
			r := o.a[i]
			o.m = append(o.m, match{b: r.matchLen, o: r.off})
			i -= int(r.matchLen)
		}
		if i < 0 {
			panic("matchLen issue")
		}

		sequences := o.blk.Sequences[:0]
		literals := o.blk.Literals[:0]
		var seq Seq
		i = 0
		for j := len(o.m) - 1; j >= 0; j-- {
			u := o.m[j]
			if u.o == 0 {
				seq.LitLen += u.b
				k = i + int(u.b)
				literals = append(literals, o.p[i:k]...)
				i = k
			} else {
				seq.Offset = u.o
				seq.MatchLen = u.b
				i += int(u.b)
				sequences = append(sequences, seq)
				seq = Seq{}
			}
		}

		if seq.LitLen > 0 {
			if o.flags&NoTrailingLiterals != 0 {
				literals = literals[:len(literals)-int(seq.LitLen)]
			} else {
				sequences = append(sequences, seq)
			}
		}

		o.blk.Sequences = sequences
		o.blk.Literals = literals

		return n
	*/

	panic("TODO")
}
