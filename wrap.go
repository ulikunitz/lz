package lz

import (
	"io"
	"reflect"
)

// Wrap combines a reader and a InputSequencer and makes a Sequencer. The user
// doesn't need to take care of filling the Sequencer with additional data. The
// returned sequencer returns EOF if no further data is available.
func Wrap(r io.Reader, wseq InputSequencer) *WrappedSequencer {
	return &WrappedSequencer{r: r, wseq: wseq}
}

// WrappedSequencer is returned by the Wrap function. It provides the Sequence
// method and reads the data required automatically from the stored reader.
type WrappedSequencer struct {
	r    io.Reader
	wseq InputSequencer
}

type memSizer interface {
	MemSize() uintptr
}

// MemSize returns the memory consumption of the wrapped sequencer.
func (s *WrappedSequencer) MemSize() uintptr {
	n := reflect.TypeOf(*s).Size()
	n += s.wseq.(memSizer).MemSize()
	return n
}

// Sequence creates a block of sequences but reads the required data from the
// reader if necessary. The function returns io.EOF if no further data is
// available.
func (s *WrappedSequencer) Sequence(blk *Block, flags int) (n int, err error) {
	if r := s.wseq.RequestBuffer(); r > 0 {
		_, err = s.wseq.ReadFrom(s.r)
	}
	var serr error
	n, serr = s.wseq.Sequence(blk, flags)
	if serr == ErrEmptyBuffer && err == nil {
		serr = io.EOF
	}
	return n, serr
}

// Reset puts the WrappedSequencer in its initial state and changes the wrapped
// reader to another reader.
func (s *WrappedSequencer) Reset(r io.Reader) {
	s.wseq.Reset()
	s.r = r
}
