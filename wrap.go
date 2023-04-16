package lz

import (
	"fmt"
	"io"
)

// Wrap combines a reader and a Sequencer and makes a Sequencer. The user
// doesn't need to take care of filling the Sequencer with additional data. The
// returned sequencer returns EOF if no further data is available.
//
// Wrap chooses the minimum of 32 kbyte or half of the window size as shrink
// size.
func Wrap(r io.Reader, seq Sequencer) *WrappedSequencer {
	cfg := seq.Config().BufferConfig()
	s := &WrappedSequencer{
		BufConfig: cfg,
		wr:           r,
		s:            seq,
		data:         make([]byte, cfg.BufferSize, cfg.BufferSize+7),
	}
	return s
}

// WrappedSequencer is returned by the Wrap function. It provides the Sequence
// method and reads the data required automatically from the stored reader.
type WrappedSequencer struct {
	BufConfig

	wr io.Reader
	err error
	s  Sequencer

	data []byte
	n    int
	w    int
}

// Sequence creates a block of sequences but reads the required data from the
// reader if necessary. The function returns io.EOF if no further data is
// available.
func (s *WrappedSequencer) Sequence(blk *Block, flags int) (n int, err error) {
	k := s.n - s.w
	if k < s.BlockSize && s.err == nil {
		if k < 0 {
			panic(fmt.Errorf("s.w=%d > s.n=%d", s.w, s.n))
		}
		// shrink
		delta := doz(s.w, s.ShrinkSize)
		if delta > 0 {
			s.n = copy(s.data, s.data[delta:s.n])
			s.w -= delta
		}
		r, err := io.ReadFull(s.wr, s.data[s.n:])
		s.n += r
		if err != nil {
			if err == io.ErrUnexpectedEOF {
				err = io.EOF
			}
			s.err = err
		}
		s.s.Update(s.data[:s.n], delta)
		k = s.n-s.w
	}
	if k == 0 {
		if s.err == nil {
			panic(fmt.Errorf("k == 0 && s.err == nil"))
		}
		return 0, s.err
	}
	if k > s.BlockSize {
		flags |= NoTrailingLiterals
	} else {
		flags &^= NoTrailingLiterals
	}
	n, err = s.s.Sequence(blk, flags)
	if err != nil {
		panic(fmt.Errorf("unexpected sequence error %w", err))
	}
	s.w += n
	return n, nil
}

// Reset puts the WrappedSequencer in its initial state and changes the wrapped
// reader to another reader.
func (s *WrappedSequencer) Reset(r io.Reader) {
	s.wr = r
	s.err = nil
	s.w = 0
	s.n = 0
	s.s.Update(s.data[:0], -1)
}
