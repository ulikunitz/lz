package lz

import (
	"fmt"
	"io"
)

type seqBuffer struct {
	data []byte

	w int

	windowSize int
	max        int
	shrinkSize int
}

func (buf *seqBuffer) Init(windowSize, max, shrink int) error {
	if !(windowSize >= 1) {
		return fmt.Errorf("lz: window size must be >= 1")
	}
	if !(shrink >= 0) {
		return fmt.Errorf("lz: shrink must be >= 0")
	}
	if !(shrink <= windowSize) {
		return fmt.Errorf("lz: shrink must be <= window size")
	}
	if !(windowSize <= max) {
		return fmt.Errorf("lz: maxSo must be >= window size")
	}
	*buf = seqBuffer{
		data:       buf.data[0:],
		windowSize: windowSize,
		max:        max,
		shrinkSize: shrink,
	}
	return nil
}

func (buf *seqBuffer) Reset() {
	buf.data = buf.data[:0]
	buf.w = 0
}

func (buf *seqBuffer) available() int {
	return buf.max - len(buf.data)
}

func (buf *seqBuffer) buffered() int {
	return len(buf.data) - buf.w
}

// Write writes data into the buffer that will be later processed by the
// Sequence method.
func (buf *seqBuffer) Write(p []byte) (n int, err error) {
	n = buf.available()
	if len(p) > n {
		p = p[:n]
		err = ErrFullBuffer
	}
	buf.data = append(buf.data, p...)
	return len(p), err
}

// ReadFrom is an alternative way to write data into the buffer.
func (buf *seqBuffer) ReadFrom(r io.Reader) (n int64, err error) {
	var p []byte
	if buf.max < cap(buf.data) {
		p = buf.data[:buf.max]
	} else {
		p = buf.data[:cap(buf.data)]
	}
	if len(p) == 0 {
		n := 32 * 1024
		if buf.max < n {
			n = buf.max
		}
		p = make([]byte, n)
	}
	i := len(buf.data)
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
		if i >= buf.max {
			err = ErrFullBuffer
			break
		}
		// doubling the size of data
		k = 2 * i
		if k > buf.max || k < 0 {
			k = buf.max
		}
		q := make([]byte, k)
		// don't copy data before the window starts
		r := buf.w - buf.windowSize
		if r < 0 {
			r = 0
		}
		copy(q[r:], p[r:])
		p = q
	}
	n = int64(i - len(buf.data))
	buf.data = p[:i]
	return n, err
}

// Shrink moves the tail of the Window, determined by ShrinkSize, to the front
// of the buffer and makes then more space available to write into the buffer.
func (buf *seqBuffer) Shrink() int {
	r := buf.w - buf.shrinkSize
	if r < 0 {
		r = 0
	}
	copy(buf.data, buf.data[r:])
	buf.data = buf.data[:len(buf.data)-r]
	buf.w -= r
	return r
}
