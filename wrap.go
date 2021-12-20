package lz

import (
	"errors"
	"io"
	"reflect"
)

// Wrap combines a reader and a Sequencer and makes a Sequencer. The user
// doesn't need to take care of filling the Sequencer with additional data. The
// returned sequencer returns EOF if no further data is available.
func Wrap(r io.Reader, seq Sequencer) *WrappedSequencer {
	return &WrappedSequencer{r: r, s: seq}
}

// WrappedSequencer is returned by the Wrap function. It provides the Sequence
// method and reads the data required automatically from the stored reader.
type WrappedSequencer struct {
	r io.Reader
	s Sequencer
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

func requestBuffer(s Sequencer, w *Window, blockSize int) int {
	r := blockSize - w.Buffered()
	if r <= 0 {
		return 0
	}
	if w.Available() < r {
		var ws int
		if w.size < 64<<10 {
			ws = w.size / 2
		} else {
			ws = 32 << 10
		}
		s.Shrink(ws)
	}
	return w.Available()
}

// Sequence creates a block of sequences but reads the required data from the
// reader if necessary. The function returns io.EOF if no further data is
// available.
func (s *WrappedSequencer) Sequence(blk *Block, blockSize int, flags int) (n int, err error) {
	if blockSize < 1 {
		return 0, errors.New("lz: blockSize must be >= 1")
	}
	w := s.s.WindowPtr()
	if r := requestBuffer(s.s, w, blockSize); r > 0 {
		_, err = w.ReadFrom(s.r)
	}
	var serr error
	n, serr = s.s.Sequence(blk, blockSize, flags)
	if serr == ErrEmptyBuffer && err == nil {
		serr = io.EOF
	}
	return n, serr
}

// Reset puts the WrappedSequencer in its initial state and changes the wrapped
// reader to another reader.
func (s *WrappedSequencer) Reset(r io.Reader) {
	s.s.Reset()
	s.r = r
}
