package lz

import (
	"fmt"
)

// bucketHashSequencer allows the creation of sequence blocks using a simple hash
// table.
type bucketHashSequencer struct {
	SeqBuffer

	bucketHash
}

// BUHSConfig provides the configuration parameters for the bucket hash sequencer.
type BUHSConfig struct {
	SBConfig
	// number of bits of the hash index
	HashBits int
	// length of the input used; range [2,8]
	InputLen int
	// size of a bucket; range [1,128]
	BucketSize int
}

// ApplyDefaults sets values that are zero to their defaults values.
func (cfg *BUHSConfig) ApplyDefaults() {
	cfg.SBConfig.ApplyDefaults()
	if cfg.InputLen == 0 {
		cfg.InputLen = 3
	}
	if cfg.HashBits == 0 {
		cfg.HashBits = 12
	}
	if cfg.BucketSize == 0 {
		cfg.BucketSize = 10
	}
}

// Verify checks the config for correctness.
func (cfg *BUHSConfig) Verify() error {
	if err := cfg.SBConfig.Verify(); err != nil {
		return err
	}

	if !(2 <= cfg.InputLen && cfg.InputLen <= 8) {
		return fmt.Errorf(
			"lz: InputLen=%d; must be in range [2,8]", cfg.InputLen)
	}
	if !(cfg.InputLen <= cfg.WindowSize) {
		return fmt.Errorf(
			"lz: cfg.WindowSize is %d; must be >= InputLen=%d",
			cfg.WindowSize, cfg.InputLen)
	}
	if !(int64(cfg.WindowSize) <= int64(maxUint32)) {
		// We manage positions only as uint32 values and so this limit
		// is necessary
		return fmt.Errorf(
			"lz: WindowSize=%d; must be less than MaxUint32=%d",
			cfg.WindowSize, maxUint32)
	}
	// A single hash table should not have more than 2 GByte size. Since the
	// bucket size has the maximum 128. We can support only 24 bits for rhe
	// hash size at maximum.
	maxHashBits := 24
	if t := 8 * cfg.InputLen; t < maxHashBits {
		maxHashBits = t
	}
	if !(0 <= cfg.HashBits && cfg.HashBits <= maxHashBits) {
		return fmt.Errorf("lz: HashBits=%d; must be <= %d",
			cfg.HashBits, maxHashBits)
	}
	if !(1 <= cfg.BucketSize && cfg.BucketSize <= 128) {
		return fmt.Errorf("lz: BucketSize=%d; must be in range [1,128]",
			cfg.BucketSize)
	}
	return nil
}

// NewSequencer creates a new hash sequencer.
func (cfg BUHSConfig) NewSequencer() (s Sequencer, err error) {
	return newBucketHashSequencer(cfg)
}

// newBucketHashSequencer creates a new hash sequencer.
func newBucketHashSequencer(cfg BUHSConfig) (s *bucketHashSequencer, err error) {
	s = new(bucketHashSequencer)
	if err := s.Init(cfg); err != nil {
		return nil, err
	}
	return s, nil
}

// Init initializes the hash sequencer. It returns an error if there is an issue
// with the configuration parameters.
func (s *bucketHashSequencer) Init(cfg BUHSConfig) error {
	cfg.ApplyDefaults()
	var err error
	if err = cfg.Verify(); err != nil {
		return err
	}

	err = s.SeqBuffer.Init(cfg.SBConfig)
	if err != nil {
		return err
	}
	err = s.bucketHash.init(cfg.InputLen, cfg.HashBits, cfg.BucketSize)
	if err != nil {
		return err
	}

	return nil
}

// Reset resets the hash sequencer. The sequencer will be in the same state as
// after Init.
func (s *bucketHashSequencer) Reset(data []byte) error {
	if err := s.SeqBuffer.Reset(data); err != nil {
		return err
	}
	s.bucketHash.reset()
	return nil
}

// Shrink shortens the window size to make more space available for Write and
// ReadFrom.
func (s *bucketHashSequencer) Shrink() int {
	w := s.SeqBuffer.w
	n := s.SeqBuffer.shrink()
	s.bucketHash.adapt(uint32(w - n))
	return n
}

func (s *bucketHashSequencer) hashSegment(a, b int) {
	if a < 0 {
		a = 0
	}
	c := len(s.data) - s.inputLen + 1
	if b > c {
		b = c
	}

	// Ensure that we can use _getLE64 all the time.
	_p := s.data[:b+7]

	for i := a; i < b; i++ {
		x := _getLE64(_p[i:]) & s.mask
		s.bucketHash.add(s.hashValue(x), uint32(i), uint32(x))
	}
}

// Sequence converts the next block to sequences. The contents of the blk
// variable will be overwritten. The method returns the number of bytes
// sequenced and any error encountered. It return ErrEmptyBuffer if there is no
// further data available.
//
// If blk is nil the search structures will be filled. This mode can be used to
// ignore segments of data.
func (s *bucketHashSequencer) Sequence(blk *Block, flags int) (n int, err error) {
	n = s.Buffered()
	if n > s.BlockSize {
		n = s.BlockSize
	}

	if blk == nil {
		if n == 0 {
			return 0, ErrEmptyBuffer
		}
		t := s.w + n
		s.hashSegment(s.w-s.inputLen+1, t)
		s.w = t
		return n, nil

	}

	blk.Sequences = blk.Sequences[:0]
	blk.Literals = blk.Literals[:0]

	if n == 0 {
		return 0, ErrEmptyBuffer
	}

	s.hashSegment(s.w-s.inputLen+1, s.w)
	p := s.data[:s.w+n]

	inputEnd := len(p) - s.inputLen + 1
	i := s.w
	litIndex := i

	// Ensure that we can use _getLE64 all the time.
	_p := s.data[:inputEnd+7]

	for ; i < inputEnd; i++ {
		x := _getLE64(_p[i:]) & s.mask
		h := s.hashValue(x)
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
			ke := matchLen(p[j:], p[i:])
			if ke < k || (ke == k && oe >= o) {
				continue
			}
			o, k = oe, ke
		}
		s.add(h, uint32(i), v)
		if k < 2 {
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
			h := s.hashValue(x)
			s.add(h, uint32(j), uint32(x))
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
	return n, nil
}
