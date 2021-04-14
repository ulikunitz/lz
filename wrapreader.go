package lz

import (
	"io"
)

// Wrap combines a reader and a WriteSequencer and makes a Sequencer. The user
// doesn't need to take care of filling the Sequencer with additional data. The
// returned sequencer returns EOF if no further data is available.
func Wrap(r io.Reader, wseq WriteSequencer) *WrappedSequencer {
	return &WrappedSequencer{r: r, wseq: wseq}
}

type WrappedSequencer struct {
	r    io.Reader
	wseq WriteSequencer
}

// Sequence creates a block of sequences but reads the required data from the
// reader if necessary. The function returns io.EOF if no further data is
// available.
func (s *WrappedSequencer) Sequence(blk *Block, flags int) (n int, err error) {
	if r := s.wseq.Requested(); r > 0 {
		// We are reading as much bytes as we can. Copy returns nil if
		// s.r has reached io.EOF.
		_, err = io.Copy(s.wseq, s.r)
	}
	var serr error
	n, serr = s.wseq.Sequence(blk, flags)
	if serr == ErrEmptyBuffer && err == nil {
		serr = io.EOF
	}
	return n, serr
}

func (s *WrappedSequencer) Reset(r io.Reader) {
	s.wseq.Reset()
	s.r = r
}
