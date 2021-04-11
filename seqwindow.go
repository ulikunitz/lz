package lz

import (
	"fmt"
	"io"
)

const maxUint32 = 1<<32 - 1

type seqWindow struct {
	data []byte

	// pos at the start of data; pos+max <= 1<<32
	pos uint32

	// window head
	w int
	// window size
	size int
	// maximum size that data can grow
	max int
	// size to which the window will resized after the maximum data size has
	// been reached
	shrinkSize int
}

func (w *seqWindow) init(size int, max int, shrink int) error {
	if !(size >= 1) {
		return fmt.Errorf("lz: window size must be >= 1")
	}
	if !(shrink >= 0) {
		return fmt.Errorf("lz: shrink must be >= 0")
	}
	if !(shrink <= size) {
		return fmt.Errorf("lz: shrink must be <= window size")
	}
	if !(size <= max) {
		return fmt.Errorf("lz: max must be >= window size")
	}
	if !(max <= maxUint32) {
		return fmt.Errorf("lz: max is larger than maxUint32")
	}
	*w = seqWindow{
		data:       w.data[:0],
		size:       size,
		max:        max,
		shrinkSize: shrink,
	}
	return nil
}

func (w *seqWindow) available() int {
	return w.max - len(w.data)
}

// Buffered returns the number of bytes that have not been transferred into the
// window.
func (w *seqWindow) Buffered() int {
	return len(w.data) - w.w
}

// Write puts data into the buffer behind the window. This data is required by
// the Sequence method.
func (w *seqWindow) Write(p []byte) (n int, err error) {
	n = w.available()
	if len(p) > n {
		p = p[:n]
		err = ErrBufferFull
	}
	w.data = append(w.data, p...)
	return len(p), err
}

// ReadFrom is an alternative way to transfer data into the buffer after the
// window. See the Write method.
func (w *seqWindow) ReadFrom(r io.Reader) (n int64, err error) {
	var p []byte
	if w.max < cap(w.data) {
		p = w.data[:w.max]
	} else {
		p = w.data[:cap(w.data)]
	}
	i := len(w.data)
	for {
		var k int
		k, err = r.Read(p[i:])
		i += k
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			break
		}
		if i < len(p) {
			// p is not exhausted
			continue
		}
		if i >= w.max {
			err = ErrBufferFull
			break
		}
		// doubling the size of data
		k = 2 * i
		if k > w.max || k < 0 {
			k = w.max
		}
		q := make([]byte, k)
		// don't copy data before the window starts
		r := w.w - w.size
		if r < 0 {
			r = 0
		}
		copy(q[r:], p[r:])
		p = q
	}
	n = int64(i - len(w.data))
	w.data = p[:i]
	return n, err
}

/*
// shrink moves part of the window and all buffered data to the front of data.
// The new window size will be shrinkSize. If w.pos is reset due to the limited
// range of uint32 a non-zero delta will be returned.
func (w *seqWindow) shrink() (delta uint32) {
	r := w.w - w.shrinkSize
	if r < 0 {
		r = 0
	}
	copy(w.data, w.data[r:])
	w.data = w.data[:len(w.data)-r]
	w.w -= r
	w.pos += uint32(r)
	if int64(w.pos)+int64(w.max) > maxUint32 {
		delta := w.pos
		w.pos = 0
		return delta
	}
	return 0
}
*/
