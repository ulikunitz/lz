// SPDX-FileCopyrightText: © 2021 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

package lz

import (
	"math/bits"
)

// hashParser allows the creation of sequence blocks using a simple hash
// table.
type hashParser struct {
	hashDictionary

	HPConfig
}

// HPConfig provides the configuration parameters for the
// HashParser. Parser doesn't use ShrinkSize and and BufferSize itself,
// but it provides it to other code that have to handle the buffer.
type HPConfig struct {
	ShrinkSize int
	BufferSize int
	WindowSize int
	BlockSize  int

	InputLen int
	HashBits int
}

// Clone creates a copy of the configuration.
func (cfg *HPConfig) Clone() ParserConfig {
	x := *cfg
	return &x
}

// UnmarshalJSON converts the JSON into the HPConfig structure.
func (cfg *HPConfig) UnmarshalJSON(p []byte) error {
	*cfg = HPConfig{}
	return unmarshalJSON(cfg, "HP", p)
}

// MarshalJSON creates the JSON string for the configuration. Note that it adds
// a property Type with value "HP" to the structure.
func (cfg *HPConfig) MarshalJSON() (p []byte, err error) {
	return marshalJSON(cfg, "HP")
}

// BufConfig returns the [BufConfig] value containing the buffer parameters.
func (cfg *HPConfig) BufConfig() BufConfig {
	bc := bufferConfig(cfg)
	return bc
}

// SetBufConfig sets the buffer configuration parameters of the parser
// configuration.
func (cfg *HPConfig) SetBufConfig(bc BufConfig) {
	setBufferConfig(cfg, bc)
}

// SetDefaults sets values that are zero to their defaults values.
func (cfg *HPConfig) SetDefaults() {
	bc := bufferConfig(cfg)
	bc.SetDefaults()
	setBufferConfig(cfg, bc)
	h, _ := hashCfg(cfg)
	h.SetDefaults()
	setHashCfg(cfg, h)
}

// Verify checks the configuration for correctness.
func (cfg *HPConfig) Verify() error {
	bc := bufferConfig(cfg)
	var err error
	if err = bc.Verify(); err != nil {
		return err
	}
	h, _ := hashCfg(cfg)
	err = h.Verify()
	return err
}

// NewParser creates a new hash parser.
func (cfg HPConfig) NewParser() (s Parser, err error) {
	hs := new(hashParser)
	if err = hs.init(cfg); err != nil {
		return nil, err
	}
	return hs, nil
}

// init initializes the hash parser. It returns an error if there is an issue
// with the configuration parameters.
func (s *hashParser) init(cfg HPConfig) error {
	cfg.SetDefaults()
	var err error
	if err = cfg.Verify(); err != nil {
		return err
	}

	hc, _ := hashCfg(&cfg)
	bc := bufferConfig(&cfg)
	if err = s.hashDictionary.init(hc, bc); err != nil {
		return err
	}

	s.HPConfig = cfg
	return nil
}

// ParserConfig returns the [HPConfig].
func (s *hashParser) ParserConfig() ParserConfig {
	return &s.HPConfig
}

// Parse converts the next block to sequences. The contents of the blk variable
// will be overwritten. The method returns the number of bytes sequenced and any
// error encountered. It returns ErrEmptyBuffer if there is no further data
// available.
//
// If blk is nil the internal hash will be filled. This mode can be used to
// ignore segments of data.
func (s *hashParser) Parse(blk *Block, flags int) (n int, err error) {
	n = len(s.Data) - s.W
	if n > s.BlockSize {
		n = s.BlockSize
	}

	if blk == nil {
		if n == 0 {
			return 0, ErrEmptyBuffer
		}
		t := s.W + n
		s.processSegment(s.W-s.hash.inputLen+1, t)
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
	var minMatchLen int
	if s.inputLen < 3 {
		minMatchLen = s.inputLen
	} else {
		minMatchLen = 3
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

	// len(blk.Sequences) > 0 checks that the literals are actually trailing
	// a sequence. If there is not a single sequence found, then we have to
	// add all literals to make progress.
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
