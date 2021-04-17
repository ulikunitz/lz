package lz

import (
	"fmt"
	"io"
)

type Buffer struct {
	data []byte

	w int

	windowSize int
	max        int
	shrinkSize int
}

func (buf *Buffer) Init(windowSize, max, shrink int) error {
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
	*buf = Buffer{
		data:       buf.data[0:],
		windowSize: windowSize,
		max:        max,
		shrinkSize: shrink,
	}
	return nil
}

func (buf *Buffer) Reset() {
	buf.data = buf.data[:0]
	buf.w = 0
}

func (buf *Buffer) available() int {
	return buf.max - len(buf.data)
}

func (buf *Buffer) buffered() int {
	return len(buf.data) - buf.w
}

func (buf *Buffer) Write(p []byte) (n int, err error) {
	n = buf.available()
	if len(p) > n {
		p = p[:n]
		err = ErrBufferFull
	}
	buf.data = append(buf.data, p...)
	return len(p), err
}

// ReadFrom is an alternative way to transfer data into the buffer after the
// window. See the Write method.
func (buf *Buffer) ReadFrom(r io.Reader) (n int64, err error) {
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
			err = ErrBufferFull
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

func (buf *Buffer) ShrinkBuffer() int {
	r := buf.w - buf.shrinkSize
	if r < 0 {
		r = 0
	}
	copy(buf.data, buf.data[r:])
	buf.data = buf.data[:len(buf.data)-r]
	buf.w -= r
	return r
}

const maxUint32 = 1<<32 - 1

type posBuffer struct {
	Buffer

	// pos at the start of data; pos+max <= 1<<32
	pos uint32
}

func (s *posBuffer) Init(size, max, shrink int) error {
	if err := s.Buffer.Init(size, max, shrink); err != nil {
		return err
	}
	if !(max <= maxUint32) {
		return fmt.Errorf("lz: max is larger than maxUint32")
	}
	s.pos = 0
	return nil
}

func (s *posBuffer) Reset() {
	s.Buffer.Reset()
	s.pos = 0
}

// Shrink moves part of the window and all buffered data to the front of data.
// The new window size will be shrinkSize. If w.pos is reset due to the limited
// range of uint32 a non-zero delta will be returned.
func (s *posBuffer) shrink() (delta uint32) {
	r := s.Buffer.ShrinkBuffer()
	s.pos += uint32(r)
	if int64(s.pos)+int64(s.max) <= maxUint32 {
		return 0
	}
	delta = s.pos
	s.pos = 0
	return delta
}
