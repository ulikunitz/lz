package lz

import (
	"fmt"
	"math/bits"
	"reflect"
)

// BHSConfig provides the parameters for the backward hash sequencer.
type BHSConfig struct {
	// maximal window size
	WindowSize int
	// ShrinkSize is the size the window is shrunk too if more buffer is
	// required
	ShrinkSize int
	// BlockSize is the maximum size of a block in bytes
	BlockSize int
	// number of bits of the hash index
	HashBits int
	// length of the input used; range [2,8]
	InputLen int
}

func (cfg *BHSConfig) windowConfig() WindowConfig {
	return WindowConfig{
		WindowSize: cfg.WindowSize,
		ShrinkSize: cfg.ShrinkSize,
		BlockSize:  cfg.BlockSize,
	}
}

// NewSequencer create a new backward hash sequencer.
func (cfg BHSConfig) NewSequencer() (s Sequencer, err error) {
	return NewBackwardHashSequencer(cfg)
}

// ApplyDefaults sets values that are zero to their defaults values.
func (cfg *BHSConfig) ApplyDefaults() {
	wcfg := cfg.windowConfig()
	wcfg.ApplyDefaults()
	cfg.WindowSize = wcfg.WindowSize
	cfg.ShrinkSize = wcfg.ShrinkSize
	cfg.BlockSize = wcfg.BlockSize
	if cfg.InputLen == 0 {
		cfg.InputLen = 3
	}
	if cfg.HashBits == 0 {
		cfg.HashBits = 12
	}
}

// Verify checks the config for correctness.
func (cfg *BHSConfig) Verify() error {
	wcfg := cfg.windowConfig()
	if err := wcfg.Verify(); err != nil {
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
	maxHashBits := 32
	if t := 8 * cfg.InputLen; t < maxHashBits {
		maxHashBits = t
	}
	if !(0 <= cfg.HashBits && cfg.HashBits <= maxHashBits) {
		return fmt.Errorf("lz: HashBits=%d; must be <= %d",
			cfg.HashBits, maxHashBits)
	}
	return nil
}

// BackwardHashSequencer allows the creation of sequence blocks using a simple
// hash table. It extends found matches by looking backward in the input stream.
type BackwardHashSequencer struct {
	Window

	hash
}

// WindowPtr returns the pointer to the window structure.
func (s *BackwardHashSequencer) WindowPtr() *Window { return &s.Window }

// MemSize returns the consumed memory size by the sequencer.
func (s *BackwardHashSequencer) MemSize() uintptr {
	n := reflect.TypeOf(*s).Size()
	n += s.Window.additionalMemSize()
	n += s.hash.additionalMemSize()
	return n
}

// NewBackwardHashSequencer creates a new backward hash sequencer.
func NewBackwardHashSequencer(cfg BHSConfig) (s *BackwardHashSequencer, err error) {
	var t BackwardHashSequencer
	if err := t.Init(cfg); err != nil {
		return nil, err
	}
	return &t, nil
}

// Init initializes the backward hash sequencer. It returns an error if there is
// an issue with the configuration parameters.
func (s *BackwardHashSequencer) Init(cfg BHSConfig) error {
	cfg.ApplyDefaults()
	var err error
	if err = cfg.Verify(); err != nil {
		return err
	}
	err = s.Window.Init(cfg.windowConfig())
	if err != nil {
		return err
	}
	if err = s.hash.init(cfg.InputLen, cfg.HashBits); err != nil {
		return err

	}
	return nil
}

// Reset resets the backward hash sequencer to the initial state after Init has
// returned.
func (s *BackwardHashSequencer) Reset(data []byte) error {
	if err := s.Window.Reset(data); err != nil {
		return err
	}
	s.hash.reset()
	return nil
}

// Shrink shortens the window size to make more space available for Write and
// ReadFrom.
func (s *BackwardHashSequencer) Shrink() int {
	oldWindowLen := s.Window.w
	n := s.Window.shrink()
	s.hash.adapt(uint32(oldWindowLen - n))
	return n
}

func (s *BackwardHashSequencer) hashSegment(a, b int) {
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
		h := s.hashValue(x)
		s.table[h] = hashEntry{
			pos:   uint32(i),
			value: uint32(x),
		}
	}
}

// Sequence converts the next block of k bytes to a sequences. The block will be
// overwritten. The method returns the number of bytes sequenced and any error
// encountered. It return ErrEmptyBuffer if there is no further data available.
//
// If blk is nil the search structures will be filled. This mode can be used to
// ignore segments of data.
func (s *BackwardHashSequencer) Sequence(blk *Block, flags int) (n int, err error) {
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
		y := _getLE64(_p[i:])
		x := y & s.mask
		h := s.hashValue(x)
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
		if o <= 0 {
			continue
		}
		k := bits.TrailingZeros64(_getLE64(_p[j:])^y) >> 3
		if k > len(p)-int(i) {
			k = len(p) - int(i)
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
			m := backwardMatchLen(p[j-back:j], p[:i])
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
			h := s.hashValue(x)
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
	n = int(i) - s.w
	s.w = int(i)
	return n, nil
}
