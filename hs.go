package lz

import (
	"fmt"
	"math/bits"
)

// hashSequencer allows the creation of sequence blocks using a simple hash
// table.
type hashSequencer struct {
	hashFinder

	w int

	windowSize int
	blockSize  int
}

// HSConfig provides the configuration parameters for the
// HashSequencer.
type HSConfig struct {
	BufferConfig
	HashConfig
}

// ApplyDefaults sets values that are zero to their defaults values.
func (cfg *HSConfig) ApplyDefaults() {
	cfg.BufferConfig.ApplyDefaults()
	cfg.HashConfig.ApplyDefaults()
}

// Verify checks the config for correctness.
func (cfg *HSConfig) Verify() error {
	if err := cfg.BufferConfig.Verify(); err != nil {
		return err
	}
	if err := cfg.HashConfig.Verify(); err != nil {
		return err
	}
	return nil
}

// NewSequencer creates a new hash sequencer.
func (cfg HSConfig) NewSequencer() (s Sequencer, err error) {
	hs := new(hashSequencer)
	if err = hs.init(cfg); err != nil {
		return nil, err
	}
	return hs, nil
}

// init initializes the hash sequencer. It returns an error if there is an issue
// with the configuration parameters.
func (s *hashSequencer) init(cfg HSConfig) error {
	cfg.ApplyDefaults()
	var err error
	if err = cfg.Verify(); err != nil {
		return err
	}

	if err = s.hashFinder.init(cfg.InputLen, cfg.HashBits); err != nil {
		return err
	}
	
	s.windowSize = cfg.WindowSize
	s.blockSize = cfg.BlockSize
	return nil
}

func (s *hashSequencer) Reset(data []byte, delta int) {
	if delta <= 0 {
		s.w = 0
	} else {
		s.w -= delta
		if !(0 <= s.w && s.w <= len(s.data)) {
			panic(fmt.Errorf("new s.w=%d is out of range [%d,%d]",
				s.w, 0, len(s.data)))
		}
	}
	s.hashFinder.Reset(data, delta)
}

// Sequence converts the next block to sequences. The contents of the blk
// variable will be overwritten. The method returns the number of bytes
// sequenced and any error encountered. It return ErrEmptyBuffer if there is no
// further data available.
//
// If blk is nil the search structures will be filled. This mode can be used to
// ignore segments of data.
func (s *hashSequencer) Sequence(blk *Block, flags int) (n int, head int, err error) {
	n = len(s.data) - s.w
	if n > s.blockSize {
		n = s.blockSize
	}

	if blk == nil {
		if n == 0 {
			return 0, s.w, ErrNoData
		}
		t := s.w + n
		s.ProcessSegment(s.w-s.hash.inputLen+1, t)
		s.w = t
		return n, t, nil

	}

	blk.Sequences = blk.Sequences[:0]
	blk.Literals = blk.Literals[:0]

	if n == 0 {
		return 0, s.w, ErrNoData
	}

	s.ProcessSegment(s.w-s.inputLen+1, s.w)
	p := s.data[:s.w+n]

	inputEnd := len(p) - s.inputLen + 1
	i := s.w
	litIndex := i
	var minMatchLen int
	if s.inputLen < 3 {
		minMatchLen = s.inputLen
	} else {
		minMatchLen = 3
	}

	// Ensure that we can use _getLE64 all the time.
	_p := s.data[:inputEnd+7]

	for ; i < inputEnd; i++ {
		y := _getLE64(_p[i:])
		x := y & s.mask
		h := hashValue(x, s.shift)
		entry := s.table[h]
		v := uint32(x)
		s.table[h] = hashEntry{
			pos:   uint32(i),
			value: v,
		}
		if v != entry.value {
			continue
		}
		// potential match
		j := int(entry.pos)
		o := i - j
		if !(0 < o && o <= s.windowSize) {
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
		var b int
		if litIndex > inputEnd {
			b = inputEnd
		} else {
			b = litIndex
		}
		for j = i + 1; j < b; j++ {
			x := _getLE64(_p[j:]) & s.mask
			h := hashValue(x, s.shift)
			s.table[h] = hashEntry{
				pos:   uint32(j),
				value: uint32(x),
			}
		}
		i = litIndex - 1
	}

	if flags&NoTrailingLiterals != 0 {
		i = litIndex
	} else {
		blk.Literals = append(blk.Literals, p[litIndex:]...)
		i = len(p)
	}
	n = i - s.w
	s.w = i
	return n, i, nil
}
