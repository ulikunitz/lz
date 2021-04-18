package lz

// BackwardHashSequencer allows the creation of sequence blocks using a simple
// hash table. It extends found matches by looking backward in the input stream.
type BackwardHashSequencer struct {
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

// hashes the masked x
func (s *BackwardHashSequencer) hash(x uint64) uint32 {
	return uint32((x * prime) >> s.shift)
}

// NewBackwardHashSequencer creates a new backward hash sequencer.
func NewBackwardHashSequencer(cfg HSConfig) (s *BackwardHashSequencer, err error) {
	var t BackwardHashSequencer
	if err := t.Init(cfg); err != nil {
		return nil, err
	}
	return &t, nil
}

// Init initialzes the backward hash sequencer. It returns an error if there is an issue
// with the configuration parameters.
func (s *BackwardHashSequencer) Init(cfg HSConfig) error {
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

// Reset resets the backward hash sequencer to the initial state after Init has
// returned.
func (s *BackwardHashSequencer) Reset() {
	s.seqBuffer.Reset()
	s.pos = 0
	for i := range s.hashTable {
		s.hashTable[i] = hashEntry{}
	}
}

// Requested provides the number of bytes that the sequencer requests to be
// filled.
func (s *BackwardHashSequencer) Requested() int {
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

func (s *BackwardHashSequencer) hashSegment(a, b int) {
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

// Sequence converts the next block of k bytes to a sequences. The block will be
// overwritten. The method returns the number of bytes sequenced and any error
// encountered. It return ErrEmptyBuffer if there is no further data available.
//
// If blk is nil the search structures will be filled. This mode can be used to
// ignore segments of data.
func (s *BackwardHashSequencer) Sequence(blk *Block, flags int) (n int, err error) {
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
		if back := i - litIndex; back > 0 {
			if back > j {
				back = j
			}
			m := backwardMatchLen(p[j-back:j], p[:i])
			i -= int64(m)
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
func (s *BackwardHashSequencer) WindowSize() int { return s.windowSize }