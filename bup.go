// SPDX-FileCopyrightText: © 2021 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

package lz

// bucketParser allows the creation of sequence blocks using a simple hash
// table.
type bucketParser struct {
	bucketDictionary
}

// NewBucketParser creates a new parser instance that uses the bucket hashes for
// finding matches.
func NewBucketParser(cfg BucketConfig, bcfg BufConfig) (Parser, error) {
	b := &bucketParser{}
	if err := b.init(cfg, bcfg); err != nil {
		return nil, err
	}
	return b, nil
}

// Parse converts the next block to sequences. The contents of the blk
// variable will be overwritten. The method returns the number of bytes
// sequenced and any error encountered. It return ErrEmptyBuffer if there is no
// further data available.
//
// If blk is nil the search structures will be filled. This mode can be used to
// ignore segments of data.
func (s *bucketParser) Parse(blk *Block, flags int) (n int, err error) {
	n = len(s.Data) - s.W
	if n > s.BlockSize {
		n = s.BlockSize
	}

	if blk == nil {
		if n == 0 {
			return 0, ErrEmptyBuffer
		}
		t := s.W + n
		s.processSegment(s.W-s.inputLen+1, t)
		s.W = t
		return n, nil

	}

	blk.Sequences = blk.Sequences[:0]
	blk.Literals = blk.Literals[:0]

	if n == 0 {
		return 0, ErrEmptyBuffer
	}

	s.processSegment(s.W-s.inputLen+1, s.W)
	p := s.Data[:s.W+n]

	inputEnd := len(p) - s.inputLen + 1
	i := s.W
	litIndex := i

	minMatchLen := 3
	if s.inputLen < minMatchLen {
		minMatchLen = s.inputLen
	}

	// Ensure that we can use _getLE64 all the time.
	_p := s.Data[:inputEnd+7]

	for ; i < inputEnd; i++ {
		x := _getLE64(_p[i:]) & s.mask
		h := hashValue(x, s.shift)
		v := uint32(x)
		o, k := 0, 0
		for _, e := range s.bucket(h) {
			if v != e.val {
				if e.val == 0 && e.pos == 0 {
					break
				}
				continue
			}
			j := int(e.pos)
			oe := i - j
			if !(0 < oe && oe <= s.WindowSize) {
				continue
			}
			// We are are not immediately computing the match length
			// but check a  byte, whether there is a chance to
			// find a longer match than already found.
			if k > 0 && p[j+k-1] != p[i+k-1] {
				continue
			}
			ke := lcp(p[j:], p[i:])
			if ke < k || (ke == k && oe >= o) {
				continue
			}
			o, k = oe, ke
		}
		s.add(h, uint32(i), v)
		if k < minMatchLen {
			continue
		}
		q := p[litIndex:i]
		blk.Sequences = append(blk.Sequences,
			Seq{
				LitLen:   uint32(len(q)),
				MatchLen: uint32(k),
				Offset:   uint32(o),
			})
		blk.Literals = append(blk.Literals, q...)
		litIndex = i + k
		var b int
		if litIndex > inputEnd {
			b = inputEnd
		} else {
			b = litIndex
		}
		for j := i + 1; j < b; j++ {
			x := _getLE64(_p[j:]) & s.mask
			h := hashValue(x, s.shift)
			s.add(h, uint32(j), uint32(x))
		}
		i = litIndex - 1
	}

	if flags&NoTrailingLiterals != 0 && len(blk.Sequences) > 0 {
		i = litIndex
	} else {
		blk.Literals = append(blk.Literals, p[litIndex:]...)
		i = len(p)
	}
	n = i - s.W
	s.W = i
	return n, nil
}
