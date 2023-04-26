package lz

import (
	"io"
)

// Wrap combines a reader and a Sequencer and makes a Sequencer. The user
// doesn't need to take care of filling the Sequencer with additional data. The
// returned sequencer returns EOF if no further data is available.
//
// Wrap chooses the minimum of 32 kbyte or half of the window size as shrink
// size.
func Wrap(r io.Reader, seq Sequencer) *WrappedSequencer {
	return &WrappedSequencer{r: r, s: seq}
}

// WrappedSequencer is returned by the Wrap function. It provides the Sequence
// method and reads the data required automatically from the stored reader.
type WrappedSequencer struct {
	r io.Reader
	s Sequencer
}

// Sequence creates a block of sequences but reads the required data from the
// reader if necessary. The function returns io.EOF if no further data is
// available.
func (s *WrappedSequencer) Sequence(blk *Block, flags int) (n int, err error) {
	for {
		n, err = s.s.Sequence(blk, flags)
		if err != ErrEmptyBuffer {
			return n, err
		}
		s.s.Shrink()
		if k, err := s.s.ReadFrom(s.r); k == 0 {
			if err == ErrFullBuffer {
				panic("unexpected ErrFullBuffer")
			}
			return 0, err
		}
	}
}

// Reset puts the WrappedSequencer in its initial state and changes the wrapped
// reader to another reader.
func (s *WrappedSequencer) Reset(r io.Reader) {
	if err := s.s.Reset(nil); err != nil {
		panic(err)
	}
	s.r = r
}
