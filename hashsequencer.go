package lz

import (
	"errors"
	"fmt"
)

const maxUint32 = 1<<32 - 1

// hashEntry is used for hashEntry. The value field allows a fast check whether
// a match has been found, which is cache-optimized.
type hashEntry struct {
	pos   uint32
	value uint32
}

// HashSequencer allows the creation of sequence blocks using a simple hash
// table.
type HashSequencer struct {
	seqBuffer

	hashTable []hashEntry

	// pos of the start of data in Buffer
	pos uint32

	// mask for input
	mask uint64

	// shift provides the shift required for the hash function
	shift uint

	inputLen  int
	blockSize int
}

// prime is used for hashing
const prime = 9920624304325388887

// hashes the masked x
func (s *HashSequencer) hash(x uint64) uint32 {
	return uint32((x * prime) >> s.shift)
}

// HSConfig provides the configuration parameters for the
// HashSequencer.
//
// The pos-buffer contains the sliding window. If the window reaches the end of
// the buffer parts of it needs to be moved to the front of the buffer. The
// number of bytes to be moved are defined by the shrinkSize. A shrinkSize of 0
// is supported.
type HSConfig struct {
	// maximal window size
	WindowSize int
	// size of the window if the buffer is shrinked
	ShrinkSize int
	// maximum size of the buffer
	MaxSize int
	// BlockSize: target size for a block
	BlockSize int
	// number of bits of the hash index
	HashBits int
	// length of the input used; range [2,8]
	InputLen int
}

// ApplyDefaults sets values that are zero to their defaults values.
func (cfg *HSConfig) ApplyDefaults() {
	if cfg.BlockSize == 0 {
		cfg.BlockSize = 128 * 1024
	}
	if cfg.WindowSize == 0 {
		cfg.WindowSize = 8 * 1024 * 1024
	}
	if cfg.MaxSize == 0 {
		cfg.MaxSize = 8 * 1024 * 1024
	}
	if cfg.InputLen == 0 {
		cfg.InputLen = 4
	}
	if cfg.HashBits == 0 {
		cfg.HashBits = 12
	}
}

// Verify checks the config for correctness.
func (cfg *HSConfig) Verify() error {
	if !(2 <= cfg.InputLen && cfg.InputLen <= 8) {
		return fmt.Errorf(
			"lz: InputLen=%d; must be in range [2,8]", cfg.InputLen)
	}
	if !(cfg.InputLen <= cfg.WindowSize) {
		return fmt.Errorf(
			"lz: cfg.WindowSize is %d; must be >= InputLen=%d",
			cfg.WindowSize, cfg.InputLen)
	}
	if !(0 <= cfg.ShrinkSize && cfg.ShrinkSize <= cfg.WindowSize) {
		return fmt.Errorf(
			"lz: ShrinkSize=%d; must be <= WindowSize=%d",
			cfg.ShrinkSize, cfg.WindowSize)
	}
	if !(cfg.WindowSize <= cfg.MaxSize) {
		return fmt.Errorf(
			"lz: WindowSize=%d; must be less than MaxSize=%d",
			cfg.WindowSize, cfg.MaxSize)
	}
	if !(cfg.ShrinkSize < cfg.MaxSize) {
		return fmt.Errorf(
			"ls: shrinkSize must be less than cfg.MaxSize")
	}
	if !(int64(cfg.MaxSize) <= int64(maxUint32)) {
		// We manage positions only as uint32 values and so this limit
		// is necessary
		return fmt.Errorf(
			"lz: MaxSize=%d; must be less than MaxUint32=%d",
			cfg.MaxSize, maxUint32)
	}
	if !(0 < cfg.BlockSize) {
		return fmt.Errorf(
			"lz: BlockSize=%d; must be positive", cfg.BlockSize)
	}
	maxHashBits := 32
	if t := 8 * cfg.InputLen; t < maxHashBits {
		maxHashBits = t

	}
	if !(0 <= cfg.HashBits && cfg.HashBits <= maxHashBits) {
		return fmt.Errorf("lz: HashBits=%d; must be less than %d",
			cfg.HashBits, maxHashBits)
	}
	return nil
}

// NewHashSequencer creates a new hash sequencer.
func NewHashSequencer(cfg HSConfig) (s *HashSequencer, err error) {
	var t HashSequencer
	if err := t.Init(cfg); err != nil {
		return nil, err
	}
	return &t, nil
}

// Init initialzes the hash sequencer. It returns an error if there is an issue
// with the configuration parameters.
func (s *HashSequencer) Init(cfg HSConfig) error {
	cfg.ApplyDefaults()
	var err error
	if err = cfg.Verify(); err != nil {
		return err
	}
	err = s.seqBuffer.Init(cfg.WindowSize, cfg.MaxSize, cfg.ShrinkSize)
	if err != nil {
		return err
	}

	n := 1 << cfg.HashBits
	if n <= cap(s.hashTable) {
		s.hashTable = s.hashTable[:n]
		for i := range s.hashTable {
			s.hashTable[i] = hashEntry{}
		}
	} else {
		s.hashTable = make([]hashEntry, n)
	}

	s.mask = 1<<(uint64(cfg.InputLen)*8) - 1
	s.shift = 64 - uint(cfg.HashBits)

	s.inputLen = cfg.InputLen
	s.blockSize = cfg.BlockSize
	s.pos = 0
	return nil
}

// Reset resets the hash sequencer. The sequencer will be in the same state as
// after Init.
func (s *HashSequencer) Reset() {
	s.seqBuffer.Reset()
	s.pos = 0
	for i := range s.hashTable {
		s.hashTable[i] = hashEntry{}
	}
}

// Requested provides the number of bytes that the sequencer requests to be
// provided.
func (s *HashSequencer) Requested() int {
	r := s.blockSize - s.buffered()
	if r <= 0 {
		return 0
	}
	if s.available() < r {
		s.pos += uint32(s.Shrink())
		if int64(s.pos)+int64(s.max) > maxUint32 {
			// adapt entries in hashTable since s.pos has changed.
			for i, e := range s.hashTable {
				if e.pos < s.pos {
					s.hashTable[i] = hashEntry{}
				} else {
					s.hashTable[i].pos = e.pos - s.pos
				}
			}
			s.pos = 0
		}
	}
	return s.available()
}

func (s *HashSequencer) hashSegment(a, b int) {
	if a < 0 {
		a = 0
	}
	n := len(s.data)
	c := n - s.inputLen + 1
	if b > c {
		b = c
	}

	// Ensure that we can use _getLE64 all the time.
	k := b + 8
	if k > cap(s.data) {
		var z [8]byte
		n := len(s.data)
		s.data = append(s.data, z[:k-n]...)[:n]
	}
	_p := s.data[:k]

	for i := a; i < b; i++ {
		x := _getLE64(_p[i:]) & s.mask
		h := s.hash(x)
		s.hashTable[h] = hashEntry{
			pos:   s.pos + uint32(i),
			value: uint32(x),
		}
	}
}

// ErrEmptyBuffer indicates that the buffer is empty and no more data can be
// read or processed. More data must be provided to the buffer.
var ErrEmptyBuffer = errors.New("lz: empty buffer")

// Sequence converts the next block of k bytes to a sequences. The block will be
// overwritten. The method returns the number of bytes sequenced and any error
// encountered. It return ErrEmptyBuffer if there is no further data available.
//
// If blk is nil the search structures will be filled. This mode can be used to
// ignore segments of data.
func (s *HashSequencer) Sequence(blk *Block, flags int) (n int, err error) {
	// TODO: possible optimizations
	// - use loaded 8-byte x loaded as a kind of buffer
	// - combine hashing and match determination in loop

	n = s.blockSize
	buffered := s.buffered()
	if n > buffered {
		n = buffered
	}
	if blk == nil {
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

	inputEnd := int64(len(p) - s.inputLen + 1)
	i := int64(s.w)
	litIndex := i

	// Ensure that we can use _getLE64 all the time.
	k := int(inputEnd + 8)
	if k > cap(s.data) {
		var z [8]byte
		m := len(s.data)
		s.data = append(s.data, z[:k-m]...)[:m]
	}
	_p := s.data[:k]
	m32 := 4
	if s.inputLen < m32 {
		m32 = s.inputLen
	}

	for ; i < inputEnd; i++ {
		x := _getLE64(_p[i:]) & s.mask
		h := s.hash(x)
		v := uint32(x)
		entry := s.hashTable[h]
		s.hashTable[h] = hashEntry{
			pos:   s.pos + uint32(i),
			value: v,
		}
		if v != entry.value {
			continue
		}
		// potential match
		j := int64(entry.pos) - int64(s.pos)
		// j must not be less than window start
		if j < doz64(i, int64(s.windowSize)) {
			continue
		}
		o := i - j
		if o <= 0 {
			continue
		}
		k := m32 + matchLen(p[j+int64(m32):], p[i+int64(m32):])
		q := p[litIndex:i]
		blk.Sequences = append(blk.Sequences,
			Seq{
				MatchLen: uint32(k),
				LitLen:   uint32(len(q)),
				Offset:   uint32(o),
			})
		blk.Literals = append(blk.Literals, q...)
		litIndex = i + int64(k)
		s.hashSegment(int(i+1), int(litIndex))
		i = litIndex - 1
	}

	if flags&NoTrailingLiterals != 0 {
		i = litIndex
	} else {
		blk.Literals = append(blk.Literals, p[litIndex:]...)
		i = int64(len(p))
	}
	n = int(i) - s.w
	s.w = int(i)
	return n, nil
}

// WindowSize returns the window size of the sequencer.
func (s *HashSequencer) WindowSize() int { return s.windowSize }
