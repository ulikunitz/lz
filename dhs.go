package lz

import (
	"math/bits"
)

// DHSConfig provides the configuration parameters for the DoubleHashSequencer.
type DHSConfig struct {
	BufConfig
	DHConfig
}

// Verify checks the configuration for errors.
func (cfg *DHSConfig) Verify() error {
	var err error
	if err = cfg.BufConfig.Verify(); err != nil {
		return err
	}
	if err = cfg.DHConfig.Verify(); err != nil {
		return err
	}
	return nil
}

// ApplyDefaults uses the defaults for the configuration parameters that are set
// to zero.
func (cfg *DHSConfig) ApplyDefaults() {
	cfg.BufConfig.ApplyDefaults()
	cfg.DHConfig.ApplyDefaults()
}

// NewSequencer creates a new DoubleHashSequencer.
func (cfg DHSConfig) NewSequencer() (s Sequencer, err error) {
	dhs := new(doubleHashSequencer)
	if err = dhs.init(cfg); err != nil {
		return nil, err
	}
	return dhs, nil
}

// doubleHashSequencer generates LZ77 sequences by using two hash tables. The
// input length for the two hash tables will be different. The speed of the hash
// sequencer is slower than sequencers using a single hash, but the compression
// ratio is much better.
type doubleHashSequencer struct {
	doubleHashFinder

	w int

	DHSConfig
}

// init initializes the DoubleHashSequencer. The first error found in the
// configuration will be returned.
func (s *doubleHashSequencer) init(cfg DHSConfig) error {
	cfg.ApplyDefaults()
	var err error
	if err = cfg.Verify(); err != nil {
		return err
	}

	f := &s.doubleHashFinder
	err = f.h1.init(cfg.H1.InputLen, cfg.H1.HashBits)
	if err != nil {
		return err
	}
	err = f.h2.init(cfg.H2.InputLen, cfg.H2.HashBits)
	if err != nil {
		return err
	}

	s.DHSConfig = cfg
	return nil
}

func (s *doubleHashSequencer) Update(data []byte, delta int) {
	switch {
	case delta > 0:
		s.w = doz(s.w, delta)
	case delta < 0 || s.w > len(data):
		s.w = 0
	}
	s.doubleHashFinder.Update(data, delta)
}

// Config returns [DHSConfig]
func (s *doubleHashSequencer) Config() SeqConfig {
	return &s.DHSConfig
}

// Sequence generates the LZ77 sequences. It returns the number of bytes covered
// by the new sequences. The block will be overwritten but the memory for the
// slices will be reused.
func (s *doubleHashSequencer) Sequence(blk *Block, flags int) (n int, err error) {
	n = len(s.data) - s.w
	if s.BlockSize < n {
		n = s.BlockSize
	}
	if blk == nil {
		if n == 0 {
			return 0, ErrNoData
		}
		t := s.w + n
		s.ProcessSegment(s.w-s.h2.inputLen+1, t)
		return n, nil
	}

	blk.Sequences = blk.Sequences[:0]
	blk.Literals = blk.Literals[:0]

	if n == 0 {
		return 0, ErrNoData
	}

	s.ProcessSegment(s.w-s.h2.inputLen+1, s.w)
	p := s.data[:s.w+n]

	e1 := len(p) - s.h1.inputLen + 1
	e2 := len(p) - s.h2.inputLen + 1
	i := s.w
	litIndex := i

	minMatchLen := 3
	if s.h1.inputLen < minMatchLen {
		minMatchLen = s.h1.inputLen
	}

	// Ensure that we can use _getLE64 all the time.
	_p := s.data[:e1+7]

	for ; i < e2; i++ {
		y := _getLE64(_p[i:])
		x := y & s.h2.mask
		h := hashValue(x, s.h2.shift)
		entry := s.h2.table[h]
		v2 := uint32(x)
		pos := uint32(i)
		s.h2.table[h] = hashEntry{pos: pos, value: v2}
		x = y & s.h1.mask
		h = hashValue(x, s.h1.shift)
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
		j := int(entry.pos)
		o := i - j
		if !(0 < o && o <= s.WindowSize) {
			continue
		}
		k := bits.TrailingZeros64(_getLE64(_p[j:])^y) >> 3
		if k > len(p)-i {
			k = len(p) - i
		}
		if k < minMatchLen {
			continue
		}
		if k == 8 {
			r := p[j+8:]
			q := p[i+8:]
			for len(q) >= 8 {
				x := _getLE64(r) ^ _getLE64(q)
				b := bits.TrailingZeros64(x) >> 3
				k += b
				if b < 8 {
					goto match
				}
				r = r[8:]
				q = q[8:]
			}
			if len(q) > 0 {
				x := getLE64(r) ^ getLE64(q)
				b := bits.TrailingZeros64(x) >> 3
				if b > len(q) {
					b = len(q)
				}
				k += b
			}
		match:
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
		b := litIndex
		if litIndex > e2 {
			b = e2
		}
		for j = i + 1; j < b; j++ {
			y := _getLE64(_p[j:])
			x := y & s.h2.mask
			h := hashValue(x, s.h2.shift)
			pos := uint32(j)
			s.h2.table[h] = hashEntry{pos: pos, value: uint32(x)}
			x = y & s.h1.mask
			h = hashValue(x, s.h1.shift)
			s.h1.table[h] = hashEntry{pos: pos, value: uint32(x)}
		}
		if j < litIndex {
			b = litIndex
			if litIndex > e1 {
				b = e1
			}
			for ; j < b; j++ {
				x := _getLE64(_p[j:]) & s.h1.mask
				h := hashValue(x, s.h1.shift)
				s.h1.table[h] = hashEntry{
					pos:   uint32(j),
					value: uint32(x),
				}
			}
		}
		i = litIndex - 1
	}
	for ; i < e1; i++ {
		y := _getLE64(_p[i:])
		x := y & s.h1.mask
		h := hashValue(x, s.h1.shift)
		entry := s.h1.table[h]
		v1 := uint32(x)
		s.h1.table[h] = hashEntry{
			pos:   uint32(i),
			value: v1,
		}
		if v1 != entry.value {
			continue
		}
		// potential match
		j := int(entry.pos)
		o := i - j
		if !(0 < o && o <= s.WindowSize) {
			continue
		}
		k := bits.TrailingZeros64(_getLE64(_p[j:])^y) >> 3
		if k > len(p)-i {
			k = len(p) - i
		}
		if k < minMatchLen {
			continue
		}
		if k == 8 {
			r := p[j+8:]
			q := p[i+8:]
			for len(q) >= 8 {
				x := _getLE64(r) ^ _getLE64(q)
				b := bits.TrailingZeros64(x) >> 3
				k += b
				if b < 8 {
					goto match1
				}
				r = r[8:]
				q = q[8:]
			}
			if len(q) > 0 {
				x := getLE64(r) ^ getLE64(q)
				b := bits.TrailingZeros64(x) >> 3
				if b > len(q) {
					b = len(q)
				}
				k += b
			}
		match1:
		}
		q := p[litIndex:i]
		blk.Sequences = append(blk.Sequences,
			Seq{
				MatchLen: uint32(k),
				LitLen:   uint32(len(q)),
				Offset:   uint32(o),
			})
		blk.Literals = append(blk.Literals, q...)
		litIndex = i + k
		b := litIndex
		if b > e1 {
			b = e1
		}
		for ; j < b; j++ {
			x := _getLE64(_p[j:]) & s.h1.mask
			h := hashValue(x, s.h1.shift)
			s.h1.table[h] = hashEntry{
				pos:   uint32(j),
				value: uint32(x),
			}
		}
		i = litIndex - 1
	}

	if flags&NoTrailingLiterals != 0 && len(blk.Sequences) > 0 {
		i = litIndex
	} else {
		blk.Literals = append(blk.Literals, p[litIndex:]...)
		i = len(p)
	}
	n = int(i) - s.w
	s.w = int(i)
	return n, nil
}
