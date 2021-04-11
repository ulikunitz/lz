package lz

import (
	"io"
)

// WrapReader wraps a reader. The user doesn't need to take care of filling the
// Sequencer with additional data. The returned sequencer returns EOF if no
// further data is available.
func WrapReader(r io.Reader, wseq WriteSequencer) Sequencer {
	return &wrappedSequencer{r: r, wseq: wseq}
}

type wrappedSequencer struct {
	r    io.Reader
	wseq WriteSequencer
}

// Sequence creates a block of sequences but reads the required data from the
// reader if necessary. The function returns io.EOF if no further data is
// available.
func (s *wrappedSequencer) Sequence(blk *Block, flags int) (n int, err error) {
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
