// SPDX-FileCopyrightText: © 2021 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

package lz2

// bucketHashSequencer allows the creation of sequence blocks using a simple hash
// table.
type bucketHashSequencer struct {
	buhFinder

	w int

	BUHSConfig
}

// BUHSConfig provides the configuration parameters for the bucket hash sequencer.
type BUHSConfig struct {
	ShrinkSize int
	BufferSize int
	WindowSize int
	BlockSize  int

	InputLen   int
	HashBits   int
	BucketSize int
}

// ApplyDefaults sets values that are zero to their defaults values.
func (cfg *BUHSConfig) ApplyDefaults() {
	bc := BufferConfig(cfg)
	bc.ApplyDefaults()
	SetBufferConfig(cfg, bc)
	b, _ := buhCfg(cfg)
	b.ApplyDefaults()
	setBUHCfg(cfg, b)
}

// Verify checks the config for correctness.
func (cfg *BUHSConfig) Verify() error {
	var err error
	bc := BufferConfig(cfg)
	if err = bc.Verify(); err != nil {
		return err
	}
	b, _ := buhCfg(cfg)
	if err = b.Verify(); err != nil {
		return err
	}
	return nil
}

// NewSequencer creates a new hash sequencer.
func (cfg BUHSConfig) NewSequencer() (s Sequencer, err error) {
	buhs := new(bucketHashSequencer)
	if err = buhs.init(cfg); err != nil {
		return nil, err
	}
	return buhs, nil
}

// init initializes the hash sequencer. It returns an error if there is an issue
// with the configuration parameters.
func (s *bucketHashSequencer) init(cfg BUHSConfig) error {
	cfg.ApplyDefaults()
	var err error
	if err = cfg.Verify(); err != nil {
		return err
	}

	b, _ := buhCfg(&cfg)
	if err = s.buhFinder.init(&b); err != nil {
		return err
	}

	s.BUHSConfig = cfg
	return nil
}

func (s *bucketHashSequencer) Update(data []byte, delta int) {
	switch {
	case delta > 0:
		s.w = doz(s.w, delta)
	case delta < 0 || s.w > len(data):
		s.w = 0
	}
	s.buhFinder.Update(data, delta)
}

// Config returns the [BUHSConfig].
func (s *bucketHashSequencer) Config() SeqConfig {
	return &s.BUHSConfig
}

// Sequence converts the next block to sequences. The contents of the blk
// variable will be overwritten. The method returns the number of bytes
// sequenced and any error encountered. It return ErrEmptyBuffer if there is no
// further data available.
//
// If blk is nil the search structures will be filled. This mode can be used to
// ignore segments of data.
func (s *bucketHashSequencer) Sequence(blk *Block, flags int) (n int, err error) {
	n = len(s.data) - s.w
	if n > s.BlockSize {
		n = s.BlockSize
	}

	if blk == nil {
		if n == 0 {
			return 0, ErrNoData
		}
		t := s.w + n
		s.ProcessSegment(s.w-s.inputLen+1, t)
		s.w = t
		return n, nil

	}

	blk.Sequences = blk.Sequences[:0]
	blk.Literals = blk.Literals[:0]

	if n == 0 {
		return 0, ErrNoData
	}

	s.ProcessSegment(s.w-s.inputLen+1, s.w)
	p := s.data[:s.w+n]

	inputEnd := len(p) - s.inputLen + 1
	i := s.w
	litIndex := i

	minMatchLen := 3
	if s.inputLen < minMatchLen {
		minMatchLen = s.inputLen
	}

	// Ensure that we can use _getLE64 all the time.
	_p := s.data[:inputEnd+7]

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
	n = i - s.w
	s.w = i
	return n, nil
}
