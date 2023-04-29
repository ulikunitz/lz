package lz

import (
	"encoding/json"
	"math/bits"
)

// BDHSConfig provides the configuration parameters for the backward-looking
// double Hash Sequencer.
type BDHSConfig struct {
	ShrinkSize int
	BufferSize int
	WindowSize int
	BlockSize  int

	InputLen1 int
	HashBits1 int
	InputLen2 int
	HashBits2 int
}

// MarshalJSON creates the JSON string for the configuration. Note that it adds
// a property Name with value "BDHS" to the structure.
func (cfg *BDHSConfig) MarshalJSON() (p []byte, err error) {
	s := struct {
		Name       string
		ShrinkSize int `json:",omitempty"`
		BufferSize int `json:",omitempty"`
		WindowSize int `json:",omitempty"`
		BlockSize  int `json:",omitempty"`
		InputLen1  int `json:",omitempty"`
		HashBits1  int `json:",omitempty"`
		InputLen2  int `json:",omitempty"`
		HashBits2  int `json:",omitempty"`
	}{
		Name:       "BDHS",
		ShrinkSize: cfg.ShrinkSize,
		BufferSize: cfg.BufferSize,
		WindowSize: cfg.WindowSize,
		BlockSize:  cfg.BlockSize,
		InputLen1:  cfg.InputLen1,
		HashBits1:  cfg.HashBits1,
		InputLen2:  cfg.InputLen2,
		HashBits2:  cfg.HashBits2,
	}
	return json.Marshal(&s)
}

// BufConfig returns the [BufConfig] value containing the buffer parameters.
func (cfg *BDHSConfig) BufConfig() BufConfig {
	bc := bufferConfig(cfg)
	return bc
}

// Verify checks the configuration for errors.
func (cfg *BDHSConfig) Verify() error {
	var err error
	bc := bufferConfig(cfg)
	if err = bc.Verify(); err != nil {
		return err
	}
	d, _ := dhCfg(cfg)
	if err = d.Verify(); err != nil {
		return err
	}
	return nil
}

// SetDefaults uses the defaults for the configuration parameters that are set
// to zero.
func (cfg *BDHSConfig) SetDefaults() {
	bc := bufferConfig(cfg)
	bc.SetDefaults()
	setBufferConfig(cfg, bc)
	d, _ := dhCfg(cfg)
	d.SetDefaults()
	setDHCfg(cfg, d)
}

// NewSequencer creates a new DoubleHashSequencer.
func (cfg BDHSConfig) NewSequencer() (s Sequencer, err error) {
	bdhs := new(backwardDoubleHashSequencer)
	if err = bdhs.init(cfg); err != nil {
		return nil, err
	}
	return bdhs, nil
}

// backwardDoubleHashSequencer uses two hashes and tries to extend matches
// backward.
type backwardDoubleHashSequencer struct {
	doubleHashFinder

	BDHSConfig
}

// Config returns [BDHSConfig]
func (s *backwardDoubleHashSequencer) SeqConfig() SeqConfig {
	return &s.BDHSConfig
}

// Init initializes the sequencer. The method returns an error if the
// configuration contains inconsistencies and the sequencer remains
// uninitialized.
func (s *backwardDoubleHashSequencer) init(cfg BDHSConfig) error {
	cfg.SetDefaults()
	var err error
	if err = cfg.Verify(); err != nil {
		return err
	}

	dhc, _ := dhCfg(&cfg)
	bc := bufferConfig(&cfg)
	if err = s.doubleHashFinder.init(dhc, bc); err != nil {
		return err
	}

	s.BDHSConfig = cfg
	return nil
}

// Sequence computes the LZ77 sequence for the next block. It returns the number
// of bytes actually sequenced. ErrEmptyBuffer will be returned if there is no
// data to sequence.
func (s *backwardDoubleHashSequencer) Sequence(blk *Block, flags int) (n int, err error) {
	n = len(s.Data) - s.W
	if n > s.BlockSize {
		n = s.BlockSize
	}

	if blk == nil {
		if n == 0 {
			return 0, ErrEmptyBuffer
		}
		t := s.W + n
		s.processSegment(s.W-s.h2.inputLen+1, t)
		s.W = t
		return n, nil
	}

	blk.Sequences = blk.Sequences[:0]
	blk.Literals = blk.Literals[:0]

	if n == 0 {
		return 0, ErrEmptyBuffer
	}

	s.processSegment(s.W-s.h2.inputLen+1, s.W)

	p := s.Data[:s.W+n]

	e1 := len(p) - s.h1.inputLen + 1
	e2 := len(p) - s.h2.inputLen + 1

	i := s.W
	litIndex := i

	minMatchLen := 3
	if s.h1.inputLen < minMatchLen {
		minMatchLen = s.h1.inputLen
	}

	// Ensure that we can use _getLE64 all the time.
	_p := s.Data[:e1+7]

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
		if back := i - litIndex; back > 0 {
			if back > j {
				back = j
			}
			m := lcs(p[j-back:j], p[:i])
			i -= m
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
		litIndex = i + k
		b := litIndex
		if litIndex > e2 {
			b = e2
		}
		for j = i + 1; j < b; j++ {
			y := _getLE64(_p[j:])

			pos := uint32(j)

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
		// j must not be less than window start
		o := i - j
		if !(0 < o && o <= s.WindowSize) {
			continue
		}
		k := bits.TrailingZeros64(_getLE64(_p[j:])^y) >> 3
		if k > len(p)-int(i) {
			k = len(p) - int(i)
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
		if back := i - litIndex; back > 0 {
			if back > j {
				back = j
			}
			m := lcs(p[j-back:j], p[:i])
			i -= m
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
	n = int(i) - s.W
	s.W = int(i)
	return n, nil
}
