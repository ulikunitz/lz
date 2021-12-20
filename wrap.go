package lz

import (
	"io"
	"reflect"
)

// Wrap combines a reader and a Sequencer and makes a Sequencer. The user
// doesn't need to take care of filling the Sequencer with additional data. The
// returned sequencer returns EOF if no further data is available.
//
// Wrap chooses the minimum of 32 kbyte or half of the window size as shrink
// size.
func Wrap(r io.Reader, seq Sequencer) *WrappedSequencer {
	return &WrappedSequencer{r: r, s: seq, w: seq.WindowPtr()}
}

// WrappedSequencer is returned by the Wrap function. It provides the Sequence
// method and reads the data required automatically from the stored reader.
type WrappedSequencer struct {
	r io.Reader
	s Sequencer
	w *Window
}

type memSizer interface {
	MemSize() uintptr
}

// MemSize returns the memory consumption of the wrapped sequencer.
func (s *WrappedSequencer) MemSize() uintptr {
	n := reflect.TypeOf(*s).Size()
	n += s.s.(memSizer).MemSize()
	return n
}

func (s *WrappedSequencer) requestBuffer() int {
	r := s.w.BlockSize - s.w.Buffered()
	if r <= 0 {
		return 0
	}
	if s.w.Available() < r {
		s.s.Shrink()
	}
	return s.w.Available()
}

// Sequence creates a block of sequences but reads the required data from the
// reader if necessary. The function returns io.EOF if no further data is
// available.
func (s *WrappedSequencer) Sequence(blk *Block, flags int) (n int, err error) {
	if r := s.requestBuffer(); r > 0 {
		_, err = s.w.ReadFrom(s.r)
	}
	var serr error
	n, serr = s.s.Sequence(blk, flags)
	if serr != nil {
		if serr == ErrEmptyBuffer && err == nil {
			err = io.EOF
		} else {
			err = serr
		}
	}
	if err == ErrFullBuffer {
		err = nil
	}
	return n, err
}

// Reset puts the WrappedSequencer in its initial state and changes the wrapped
// reader to another reader.
func (s *WrappedSequencer) Reset(r io.Reader) {
	s.s.Reset()
	s.r = r
}
