package lz

import (
	"fmt"
	"io"
)

// seqBuffer provides an basic buffer for creating LZ77 sequences.
type seqBuffer struct {
	data []byte

	w int

	windowSize int
	max        int
	shrinkSize int
}

func (s *seqBuffer) additionalMemSize() uintptr {
	return uintptr(cap(s.data))
}

// Init initializes the buffer.
func (s *seqBuffer) Init(windowSize, max, shrink int) error {
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
	*s = seqBuffer{
		data:       s.data[0:],
		windowSize: windowSize,
		max:        max,
		shrinkSize: shrink,
	}
	return nil
}

// Reset puts the buffer in the state after Init. The s.data slice will be
// reused.
func (s *seqBuffer) Reset() {
	s.data = s.data[:0]
	s.w = 0
}

// WindowSize returns the configured window size for the sequencer.
func (s *seqBuffer) WindowSize() int { return s.windowSize }

// Returns the number of bytes available for buffering.
func (s *seqBuffer) available() int {
	return s.max - len(s.data)
}

func (s *seqBuffer) buffered() int {
	return len(s.data) - s.w
}

// Write writes data into the buffer that will be later processed by the
// Sequence method.
func (s *seqBuffer) Write(p []byte) (n int, err error) {
	n = s.available()
	if len(p) > n {
		p = p[:n]
		err = ErrFullBuffer
	}
	s.data = append(s.data, p...)
	return len(p), err
}

// ReadFrom is an alternative way to write data into the buffer.
func (s *seqBuffer) ReadFrom(r io.Reader) (n int64, err error) {
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
			err = ErrFullBuffer
			break
		}
		// doubling the size of data
		k = 2 * i
		if k > s.max || k < 0 {
			k = s.max
		}
		q := make([]byte, k)
		// don't copy data before the window starts
		r := s.w - s.windowSize
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

// Shrink moves the tail of the Window, determined by ShrinkSize, to the front
// of the buffer and makes then more space available to write into the buffer.
func (s *seqBuffer) Shrink() int {
	r := s.w - s.shrinkSize
	if r < 0 {
		r = 0
	}
	copy(s.data, s.data[r:])
	s.data = s.data[:len(s.data)-r]
	s.w -= r
	return r
}
