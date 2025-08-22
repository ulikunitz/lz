// SPDX-FileCopyrightText: © 2021 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

package lz

import (
	"math/bits"
)

// BHPConfig provides the parameters for the backward hash parser.
type BHPConfig struct {
	ShrinkSize int
	BufferSize int
	WindowSize int
	BlockSize  int

	InputLen int
	HashBits int
}

// Clone creates a copy of the configuration.
func (cfg *BHPConfig) Clone() ParserConfig {
	x := *cfg
	return &x
}

// UnmarshalJSON parses the JSON value and sets the fields of BHPConfig.
func (cfg *BHPConfig) UnmarshalJSON(p []byte) error {
	*cfg = BHPConfig{}
	return unmarshalJSON(cfg, p)
}

// MarshalJSON creates the JSON string for the configuration. Note that it adds
// a property Type with value "BHP" to the structure.
func (cfg *BHPConfig) MarshalJSON() (p []byte, err error) {
	return marshalJSON(cfg, "BHP")
}

// BufConfig returns the [BufConfig] value containing the buffer parameters.
func (cfg *BHPConfig) BufConfig() BufConfig {
	bc := bufConfig(cfg)
	return bc
}

// SetBufConfig sets the buffer configuration parameters of the backward hash
// parser configuration.
func (cfg *BHPConfig) SetBufConfig(bc BufConfig) {
	setBufConfig(cfg, bc)
}

// SetDefaults sets values that are zero to their defaults values.
func (cfg *BHPConfig) SetDefaults() {
	bc := bufConfig(cfg)
	bc.SetDefaults()
	setBufConfig(cfg, bc)
	h, _ := hashCfg(cfg)
	h.SetDefaults()
	setHashCfg(cfg, h)
}

// Verify checks the configuration for correctness.
func (cfg *BHPConfig) Verify() error {
	bc := bufConfig(cfg)
	var err error
	if err = bc.Verify(); err != nil {
		return err
	}
	h, _ := hashCfg(cfg)
	err = h.Verify()
	return err
}

// NewParser creates a new Backward Hash Parser.
func (cfg BHPConfig) NewParser() (s Parser, err error) {
	bhs := new(backwardHashParser)
	if err = bhs.init(cfg); err != nil {
		return nil, err
	}
	return bhs, nil
}

// backwardHashParser allows the creation of sequence blocks using a simple
// hash table. It extends found matches by looking backward in the input stream.
type backwardHashParser struct {
	hashDictionary

	BHPConfig
}

// init initializes the backward hash parser. It returns an error if there is
// an issue with the configuration parameters.
func (s *backwardHashParser) init(cfg BHPConfig) error {
	cfg.SetDefaults()
	var err error
	if err = cfg.Verify(); err != nil {
		return err
	}

	hc, _ := hashCfg(&cfg)
	bc := bufConfig(&cfg)
	if err = s.hashDictionary.init(hc, bc); err != nil {
		return err
	}

	s.BHPConfig = cfg
	return nil
}

// ParserConfig returns the [BHPConfig].
func (s *backwardHashParser) ParserConfig() ParserConfig {
	return &s.BHPConfig
}

// Parse converts the next block of k bytes to a sequences. The block will be
// overwritten. The method returns the number of bytes sequenced and any error
// encountered. It return ErrEmptyBuffer if there is no further data available.
//
// If blk is nil the search structures will be filled. This mode can be used to
// ignore segments of data.
func (s *backwardHashParser) Parse(blk *Block, flags int) (n int, err error) {
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
		if litIndex > inputEnd {
			b = inputEnd
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
