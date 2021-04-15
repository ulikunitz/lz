package lz

import (
	"fmt"
	"io"
)

const maxUint32 = 1<<32 - 1

type posBuffer struct {
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

func (s *posBuffer) init(size int, max int, shrink int) error {
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
	*s = posBuffer{
		data:       s.data[:0],
		size:       size,
		max:        max,
		shrinkSize: shrink,
	}
	return nil
}

func (s *posBuffer) reset() {
	s.data = s.data[:0]
	s.pos = 0
	s.w = 0
}

func (s *posBuffer) available() int {
	return s.max - len(s.data)
}

// Buffered returns the number of bytes that have not been transferred into the
// window.
func (s *posBuffer) buffered() int {
	return len(s.data) - s.w
}

// Write puts data into the buffer behind the window. This data is required by
// the Sequence method.
func (s *posBuffer) Write(p []byte) (n int, err error) {
	n = s.available()
	if len(p) > n {
		p = p[:n]
		err = ErrBufferFull
	}
	s.data = append(s.data, p...)
	return len(p), err
}

// ReadFrom is an alternative way to transfer data into the buffer after the
// window. See the Write method.
func (s *posBuffer) ReadFrom(r io.Reader) (n int64, err error) {
	var p []byte
	if s.max < cap(s.data) {
		p = s.data[:s.max]
	} else {
		p = s.data[:cap(s.data)]
	}
	if len(p) == 0 {
		n := 32 * 1024
		if s.max < n {
			n = s.max
		}
		p = make([]byte, n)
	}
	i := len(s.data)
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
		if i >= s.max {
			err = ErrBufferFull
			break
		}
		// doubling the size of data
		k = 2 * i
		if k > s.max || k < 0 {
			k = s.max
		}
		q := make([]byte, k)
		// don't copy data before the window starts
		r := s.w - s.size
		if r < 0 {
			r = 0
		}
		copy(q[r:], p[r:])
		p = q
	}
	n = int64(i - len(s.data))
	s.data = p[:i]
	return n, err
}

// shrink moves part of the window and all buffered data to the front of data.
// The new window size will be shrinkSize. If w.pos is reset due to the limited
// range of uint32 a non-zero delta will be returned.
func (s *posBuffer) shrink() (delta uint32) {
	r := s.w - s.shrinkSize
	if r < 0 {
		r = 0
	}
	copy(s.data, s.data[r:])
	s.data = s.data[:len(s.data)-r]
	s.w -= r
	s.pos += uint32(r)
	if int64(s.pos)+int64(s.max) > maxUint32 {
		delta := s.pos
		s.pos = 0
		return delta
	}
	return 0
}
