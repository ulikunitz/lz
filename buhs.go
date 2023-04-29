package lz

import "encoding/json"

// bucketHashSequencer allows the creation of sequence blocks using a simple hash
// table.
type bucketHashSequencer struct {
	buhFinder

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

// MarshalJSON creates the JSON string for the configuration. Note that it adds
// a property Name with value "BUHS" to the structure.
func (cfg *BUHSConfig) MarshalJSON() (p []byte, err error) {
	s := struct {
		Name       string
		ShrinkSize int `json:",omitempty"`
		BufferSize int `json:",omitempty"`
		WindowSize int `json:",omitempty"`
		BlockSize  int `json:",omitempty"`
		InputLen   int `json:",omitempty"`
		HashBits   int `json:",omitempty"`
		BucketSize int `json:",omitempty"`
	}{
		Name:       "BUHS",
		ShrinkSize: cfg.ShrinkSize,
		BufferSize: cfg.BufferSize,
		WindowSize: cfg.WindowSize,
		BlockSize:  cfg.BlockSize,
		InputLen:   cfg.InputLen,
		HashBits:   cfg.HashBits,
		BucketSize: cfg.BucketSize,
	}
	return json.Marshal(&s)
}

// BufConfig returns the [BufConfig] value containing the buffer parameters.
func (cfg *BUHSConfig) BufConfig() BufConfig {
	bc := bufferConfig(cfg)
	return bc
}

// SetDefaults sets values that are zero to their defaults values.
func (cfg *BUHSConfig) SetDefaults() {
	bc := bufferConfig(cfg)
	bc.SetDefaults()
	setBufferConfig(cfg, bc)
	b, _ := buhCfg(cfg)
	b.SetDefaults()
	setBUHCfg(cfg, b)
}

// Verify checks the config for correctness.
func (cfg *BUHSConfig) Verify() error {
	var err error
	bc := bufferConfig(cfg)
	if err = bc.Verify(); err != nil {
		return err
	}
	b, _ := buhCfg(cfg)
	err = b.Verify()
	return err
}

// NewSequencer creates a new hash sequencer.
func (cfg BUHSConfig) NewSequencer() (s Sequencer, err error) {
	buhs := new(bucketHashSequencer)
	if err = buhs.init(cfg); err != nil {
		return nil, err
	}
	return buhs, nil
}

func (s *bucketHashSequencer) SeqConfig() SeqConfig {
	return &s.BUHSConfig
}

// init initializes the hash sequencer. It returns an error if there is an issue
// with the configuration parameters.
func (s *bucketHashSequencer) init(cfg BUHSConfig) error {
	cfg.SetDefaults()
	var err error
	if err = cfg.Verify(); err != nil {
		return err
	}

	b, _ := buhCfg(&cfg)
	bc := bufferConfig(&cfg)
	if err = s.buhFinder.init(b, bc); err != nil {
		return err
	}

	s.BUHSConfig = cfg
	return nil
}

// Sequence converts the next block to sequences. The contents of the blk
// variable will be overwritten. The method returns the number of bytes
// sequenced and any error encountered. It return ErrEmptyBuffer if there is no
// further data available.
//
// If blk is nil the search structures will be filled. This mode can be used to
// ignore segments of data.
func (s *bucketHashSequencer) Sequence(blk *Block, flags int) (n int, err error) {
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
