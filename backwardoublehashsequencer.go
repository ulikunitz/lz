package lz

import (
	"math/bits"
)

type BackwardDoubleHashSequencer struct {
	seqBuffer

	h1 hash

	h2 hash

	pos uint32

	blockSize int
}

func NewBackwardDoubleHashSequencer(cfg DHSConfig) (s *BackwardDoubleHashSequencer, err error) {
	s = new(BackwardDoubleHashSequencer)
	if err = s.Init(cfg); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *BackwardDoubleHashSequencer) Init(cfg DHSConfig) error {
	cfg.ApplyDefaults()
	var err error
	if err = cfg.Verify(); err != nil {
		return err
	}

	err = s.seqBuffer.Init(cfg.WindowSize, cfg.MaxSize, cfg.ShrinkSize)
	if err != nil {
		return err
	}
	if err = s.h1.init(cfg.InputLen1, cfg.HashBits1); err != nil {
		return err
	}
	if err = s.h2.init(cfg.InputLen2, cfg.HashBits2); err != nil {
		return err
	}
	s.blockSize = cfg.BlockSize
	s.pos = 0
	return nil
}

func (s *BackwardDoubleHashSequencer) Reset() {
	s.seqBuffer.Reset()
	s.h1.reset()
	s.h2.reset()
	s.pos = 0
}

func (s *BackwardDoubleHashSequencer) WindowSize() int { return s.windowSize }

func (s *BackwardDoubleHashSequencer) Requested() int {
	r := s.blockSize - s.buffered()
	if r <= 0 {
		return 0
	}
	if s.available() < r {
		s.pos += uint32(s.Shrink())
		if int64(s.pos)+int64(s.max) > maxUint32 {
			s.h1.adapt(s.pos)
			s.h2.adapt(s.pos)
			s.pos = 0
		}
	}
	return s.available()
}

func (s *BackwardDoubleHashSequencer) hashSegment1(a, b int) {
	if a < 0 {
		a = 0
	}
	n := len(s.data)
	e1 := n - s.h1.inputLen + 1
	if b < e1 {
		e1 = b
	}

	k := e1 + 8
	if k > cap(s.data) {
		var z [8]byte
		s.data = append(s.data, z[:k-n]...)[:n]
	}
	_p := s.data[:k]

	for i := a; i < e1; i++ {
		x := _getLE64(_p[i:]) & s.h1.mask
		h := s.h1.hashValue(x)
		s.h1.table[h] = hashEntry{
			pos:   s.pos + uint32(i),
			value: uint32(x),
		}
	}
}

func (s *BackwardDoubleHashSequencer) hashSegment2(a, b int) {
	if a < 0 {
		a = 0
	}
	n := len(s.data)
	e2 := n - s.h2.inputLen + 1
	if b < e2 {
		e2 = b
	}

	k := e2 + 8
	if k > cap(s.data) {
		var z [8]byte
		s.data = append(s.data, z[:k-n]...)[:n]
	}
	_p := s.data[:k]

	for i := a; i < e2; i++ {
		x := _getLE64(_p[i:]) & s.h2.mask
		h := s.h2.hashValue(x)
		s.h2.table[h] = hashEntry{
			pos:   s.pos + uint32(i),
			value: uint32(x),
		}
	}
}

func (s *BackwardDoubleHashSequencer) Sequence(blk *Block, flags int) (n int, err error) {
	n = s.blockSize
	buffered := s.buffered()
	if n > buffered {
		n = buffered
	}
	if blk == nil {
		t := s.w + n
		s.hashSegment1(s.w-s.h1.inputLen+1, t)
		s.hashSegment2(s.w-s.h2.inputLen+1, t)
		s.w = t
		return n, nil
	}
	blk.Sequences = blk.Sequences[:0]
	blk.Literals = blk.Literals[:0]

	if n == 0 {
		return 0, ErrEmptyBuffer
	}

	s.hashSegment1(s.w-s.h1.inputLen+1, s.w)
	s.hashSegment2(s.w-s.h2.inputLen+1, s.w)
	p := s.data[:s.w+n]

	e1 := int64(len(p) - s.h1.inputLen + 1)
	e2 := int64(len(p) - s.h2.inputLen + 2)
	i := int64(s.w)
	litIndex := i

	// Ensure that we can use _getLE64 all the time.
	k := int(e1 + 8)
	if k > cap(s.data) {
		var z [8]byte
		m := len(s.data)
		s.data = append(s.data, z[:k-m]...)[:m]
	}
	_p := s.data[:k]

	for ; i < e2; i++ {
		y := _getLE64(_p[i:])
		x := y & s.h2.mask
		h := s.h2.hashValue(x)
		entry := s.h2.table[h]
		v2 := uint32(x)
		pos := s.pos + uint32(i)
		s.h2.table[h] = hashEntry{pos: pos, value: v2}
		x = y & s.h1.mask
		h = s.h1.hashValue(x)
		entry1 := s.h1.table[h]
		v1 := uint32(x)
		s.h1.table[h] = hashEntry{pos: pos, value: v1}
		if v2 != entry.value {
			if v1 != entry1.value {
				continue
			}
			entry = entry1
		}
		// potential match
		j := int64(entry.pos) - int64(s.pos)
		// j must not be less than window start
		if j < doz64(i, int64(s.windowSize)) {
			continue
		}
		o := i - j
		if o <= 0 {
			continue
		}
		k := bits.TrailingZeros64(_getLE64(_p[j:])^y) >> 3
		if k > len(p)-int(i) {
			k = len(p) - int(i)
		}
		if k == 8 {
			k = 8 + matchLen(p[j+8:], p[i+8:])
		}
		if back := i - litIndex; back > 0 {
			if back > j {
				back = j
			}
			m := backwardMatchLen(p[j-back:j], p[:i])
			i -= int64(m)
			k += m
		}
		q := p[litIndex:i]
		blk.Sequences = append(blk.Sequences,
			Seq{
				MatchLen: uint32(k),
				LitLen:   uint32(len(q)),
				Offset:   uint32(o),
			})
		blk.Literals = append(blk.Literals, q...)
		litIndex = i + int64(k)
		s.hashSegment1(int(i+1), int(litIndex))
		s.hashSegment2(int(i+1), int(litIndex))
		i = litIndex - 1
	}
	for ; i < e1; i++ {
		y := _getLE64(_p[i:])
		x := y & s.h1.mask
		h := s.h1.hashValue(x)
		entry := s.h1.table[h]
		v1 := uint32(x)
		s.h1.table[h] = hashEntry{
			pos:   s.pos + uint32(i),
			value: v1,
		}
		if v1 != entry.value {
			continue
		}
		// potential match
		j := int64(entry.pos) - int64(s.pos)
		// j must not be less than window start
		if j < doz64(i, int64(s.windowSize)) {
			continue
		}
		o := i - j
		if o <= 0 {
			continue
		}
		k := bits.TrailingZeros64(_getLE64(_p[j:])^y) >> 3
		if k > len(p)-int(i) {
			k = len(p) - int(i)
		}
		if k == 8 {
			k = 8 + matchLen(p[j+8:], p[i+8:])
		}
		if back := i - litIndex; back > 0 {
			if back > j {
				back = j
			}
			m := backwardMatchLen(p[j-back:j], p[:i])
			i -= int64(m)
			k += m
		}
		q := p[litIndex:i]
		blk.Sequences = append(blk.Sequences,
			Seq{
				MatchLen: uint32(k),
				LitLen:   uint32(len(q)),
				Offset:   uint32(o),
			})
		blk.Literals = append(blk.Literals, q...)
		litIndex = i + int64(k)
		s.hashSegment1(int(i+1), int(litIndex))
		i = litIndex - 1
	}

	if flags&NoTrailingLiterals != 0 {
		i = litIndex
	} else {
		blk.Literals = append(blk.Literals, p[litIndex:]...)
		i = int64(len(p))
	}
	n = int(i) - s.w
	s.w = int(i)
	return n, nil
}
