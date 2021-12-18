package lz

import (
	"errors"
	"io"
)

// Window stores the data in the Window for encoding. New data is written into
// the window and requires a sequencer to be converted in Lempel-Ziv 77
// sequences. The literals for a particular sequences can be extracted using the
// Literals method.
//
// All data is available for matches to simplify routines.
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
	return nil
}

// Available returns the number of bytes are available for writing into the
// buffer.
func (w *Window) Available() int { return w.size - len(w.data) }

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
	if n > cap(w.data) {
		k := 2 * cap(w.data)
		if k < 1024 {
			k = 1024
		}
		if k > w.size {
			k = w.size
		}
		if n > k {
			k = n
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
		if w.size <= cap(w.data) {
			p = w.data[len(w.data):w.size]
		} else {
			p = w.data[len(w.data):cap(w.data)]
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
			k = w.size
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

// Literals extracts the byte slice of all literals defines by sequence. This
// command has been introduced to support zstd-like compression.
func (w *Window) Literals(in []byte, pos int64, seq []Seq) (literals []byte, err error) {
	ii := pos - w.start
	if !(0 <= ii && ii < int64(len(w.data))) {
		return in, errors.New("lz: pos out ouf bounds")

	}

	literals = in
	i := uint32(ii)
	for _, s := range seq {
		j := i + s.LitLen
		if j > uint32(len(w.data)) || j < i {
			return in, errors.New("lz: sequences exceed window buffer")
		}
		literals = append(literals, w.data[i:j]...)
		i = j + s.MatchLen
		if i < j {
			return in, errors.New("lz: sequence exceed window buffer")
		}
	}

	return literals, nil
}
