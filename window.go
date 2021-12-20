package lz

import (
	"errors"
	"io"
)

// Window acts as a buffer and stores the window. Data is written into the
// buffer, the sequencer creates Lempel-Ziv sequences and advances the window
// head. Since all positions behind the window head are in the window we even
// save one check in the sequencer loop.
//
// The window ensures that len(w.data)+7 < cap(w.data), which allows 64-bit
// reads on all byte position of the window.
type Window struct {
	data []byte
	// start stores the absolute position of the window
	start int64
	// w is the position of the window head in data
	w int
	// size is the window size
	size int
}

// Init initializes the window. The parameter size must be positive.
func (w *Window) Init(size int) error {
	if size <= 0 {
		return errors.New("lz: window size must be positive")
	}
	*w = Window{
		data: w.data[:0],
		size: size,
	}
	if cap(w.data) < 7 {
		w.data = make([]byte, 0, 1024)
	}
	return nil
}

// Reset cleans the window structure for reuse.
func (w *Window) Reset() {
	*w = Window{
		data: w.data[:0],
		size: w.size,
	}
	if cap(w.data) < 7 {
		panic("unexpected capacity after Init")
	}
}

// Available returns the number of bytes are available for writing into the
// buffer.
func (w *Window) Available() int { return w.size - len(w.data) }

// Buffered returns the number of bytes buffered but are not yet part of the
// window. They have to be sequenced first.
func (w *Window) Buffered() int { return len(w.data) - w.w }

// Len returns the actual length of the current window
func (w *Window) Len() int { return w.w }

// Size returns the maximum size of the window
func (w *Window) Size() int { return w.size }

// Pos returns the absolute position of the window head
func (w *Window) Pos() int64 { return w.start + int64(w.w) }

// shrink reduces the current window length to n if possible. The method returns
// the actual window length after shrinking.
func (w *Window) shrink(n int) int {
	if n < 0 {
		n = 0
	}
	if n > w.size {
		n = w.size
	}

	r := w.w - n
	if r <= 0 {
		return w.w
	}

	k := copy(w.data, w.data[r:])
	w.data = w.data[:k]
	w.start += int64(r)
	w.w = n
	return w.w
}

// ErrFullBuffer indicates that the buffer is full and no further data can be
// written.
var ErrFullBuffer = errors.New("lz: full buffer")

// Write puts data into the window. It will return ErrFullBuffer
func (w *Window) Write(p []byte) (n int, err error) {
	n = w.Available()
	if n < len(p) {
		p = p[:n]
		err = ErrFullBuffer
	}
	n = len(w.data) + len(p)
	if n+7 > cap(w.data) {
		k := 2 * cap(w.data)
		if k < 1024 {
			k = 1024
		}
		if k > w.size {
			k = w.size + 7
		}
		if n+7 > k {
			k = n + 7
		}

		t := make([]byte, n, k)
		copy(t, w.data)
		copy(t[len(w.data):], p)
		w.data = t
	} else {
		w.data = append(w.data, p...)
	}
	return len(p), err
}

// ReadFrom transfers data from the reader into the buffer.
func (w *Window) ReadFrom(r io.Reader) (n int64, err error) {
	if len(w.data) == w.size {
		return 0, ErrFullBuffer
	}
	for {
		var p []byte
		if w.size <= cap(w.data)-7 {
			p = w.data[len(w.data):w.size]
		} else {
			p = w.data[len(w.data) : cap(w.data)-7]
		}
		for len(p) > 0 {
			k, err := r.Read(p)
			n += int64(k)
			w.data = w.data[:len(w.data)+k]
			if err != nil {
				if err == io.EOF {
					err = nil
				}
				return n, err
			}
			p = p[k:]
		}
		if len(w.data) == w.size {
			return n, ErrFullBuffer
		}
		k := 2 * cap(w.data)
		if k < 1024 {
			k = 1024
		}
		if k > w.size {
			k = w.size + 7
		}
		t := make([]byte, len(w.data), k)
		copy(t, w.data)
		w.data = t
	}
}

// ByteAt returns the byte at pos unless pos is outside of the data stored in
// window.
func (w *Window) ByteAt(pos int64) (c byte, err error) {
	pos -= w.start
	if !(0 <= pos && pos < int64(len(w.data))) {
		return 0, errors.New("lz: pos outside of window buffer")
	}
	return w.data[pos], nil
}

func (w *Window) additionalMemSize() uintptr {
	return uintptr(cap(w.data))
}
