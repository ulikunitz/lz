// SPDX-FileCopyrightText: © 2021 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

package lz

// bucketParser allows the creation of sequence blocks using a simple hash
// table.
type bucketParser struct {
	bucketDictionary

	BUPConfig
}

// BUPConfig provides the configuration parameters for the bucket hash parser.
type BUPConfig struct {
	ShrinkSize int
	BufferSize int
	WindowSize int
	BlockSize  int

	InputLen   int
	HashBits   int
	BucketSize int
}

// Clone creates a copy of the configuration.
func (cfg *BUPConfig) Clone() ParserConfig {
	x := *cfg
	return &x
}

// UnmarshalJSON parses the JSON value and sets the fields of BUPConfig.
func (cfg *BUPConfig) UnmarshalJSON(p []byte) error {
	*cfg = BUPConfig{}
	return unmarshalJSON(cfg, p)
}

// MarshalJSON creates the JSON string for the configuration. Note that it adds
// a property Type with value "BUP" to the structure.
func (cfg *BUPConfig) MarshalJSON() (p []byte, err error) {
	return marshalJSON(cfg, "BUP")
}

// BufConfig returns the [BufConfig] value containing the buffer parameters.
func (cfg *BUPConfig) BufConfig() BufConfig {
	bc := bufConfig(cfg)
	return bc
}

// SetBufConfig sets the buffer configuration parameters of the parser
// configuration.
func (cfg *BUPConfig) SetBufConfig(bc BufConfig) {
	setBufConfig(cfg, bc)
}

// SetDefaults sets values that are zero to their defaults values.
func (cfg *BUPConfig) SetDefaults() {
	bc := bufConfig(cfg)
	bc.SetDefaults()
	setBufConfig(cfg, bc)
	b, _ := bucketCfg(cfg)
	b.SetDefaults()
	setBucketCfg(cfg, b)
}

// Verify checks the config for correctness.
func (cfg *BUPConfig) Verify() error {
	var err error
	bc := bufConfig(cfg)
	if err = bc.Verify(); err != nil {
		return err
	}
	b, _ := bucketCfg(cfg)
	err = b.Verify()
	return err
}

// NewParser creates a new hash parser.
func (cfg BUPConfig) NewParser() (s Parser, err error) {
	buhs := new(bucketParser)
	if err = buhs.init(cfg); err != nil {
		return nil, err
	}
	return buhs, nil
}

func (s *bucketParser) ParserConfig() ParserConfig {
	return &s.BUPConfig
}

// init initializes the hash parser. It returns an error if there is an issue
// with the configuration parameters.
func (s *bucketParser) init(cfg BUPConfig) error {
	cfg.SetDefaults()
	var err error
	if err = cfg.Verify(); err != nil {
		return err
	}

	b, _ := bucketCfg(&cfg)
	bc := bufConfig(&cfg)
	if err = s.bucketDictionary.init(b, bc); err != nil {
		return err
	}

	s.BUPConfig = cfg
	return nil
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
