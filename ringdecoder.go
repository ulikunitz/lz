package lz

import (
	"errors"
	"fmt"
	"io"
)

// RingBuffer supports the decoding of sequence blocks. It stores the window in
// a ring buffer. The decoded data must be read from the window and the simplest
// way to do that is the WriteTo method.
type RingBuffer struct {
	data []byte

	// reader index; start of valid data
	r int

	// writer index; end of valid data
	w int

	// fullWindow marks the situation where sw.w doesn't contain the window
	// size
	fullWindow bool
}

// Init initializes the ring buffer. The existing data slice in the ring buffer
// will be reused if it is has more or equal capacity than the windowSize+1.
func (buf *RingBuffer) Init(windowSize int) error {
	if windowSize < 1 {
		return fmt.Errorf("lz: winSize must be >= 1")
	}
	if cap(buf.data) > windowSize {
		*buf = RingBuffer{data: buf.data[:windowSize+1]}
	} else {
		*buf = RingBuffer{data: make([]byte, windowSize+1)}
	}
	return nil
}

// Read reads data from the writer. It will always try to return as much data as
// possible.
func (buf *RingBuffer) Read(p []byte) (n int, err error) {
	if buf.r > buf.w {
		n = copy(p, buf.data[buf.r:])
		p = p[n:]
		buf.r += n
		if buf.r < len(buf.data) {
			return n, nil
		}
		buf.r = 0
	}
	k := copy(p, buf.data[buf.r:buf.w])
	n += k
	buf.r += k
	return n, nil
}

// WriteTo writes data into the writer as much as it is possible.
func (buf *RingBuffer) WriteTo(w io.Writer) (n int64, err error) {
	if buf.r > buf.w {
		k, err := w.Write(buf.data[buf.r:])
		buf.r += k
		n = int64(k)
		if err != nil {
			return n, err
		}
		if buf.r != len(buf.data) {
			panic(fmt.Errorf("sw.r=%d; want len(sw.data)=%d", buf.r,
				len(buf.data)))
		}
		buf.r = 0
	}
	k, err := w.Write(buf.data[buf.r:buf.w])
	buf.r += k
	n += int64(k)
	return n, err
}

// available returns the number of bytes available for writing data to the ring
// buffer.
func (buf *RingBuffer) available() int {
	n := buf.r - buf.w - 1
	if n < 0 {
		n += len(buf.data)
	}
	return n
}

// len returns the actual size of the window.
func (buf *RingBuffer) len() int {
	if buf.fullWindow {
		return len(buf.data) - 1
	}
	return buf.w
}

// copySlice copies the slice into the ring buffer
func (buf *RingBuffer) copySlice(p []byte) {
	q := buf.data[buf.w:]
	k := copy(q, p)
	if k < len(q) {
		buf.w += k
		return
	}
	buf.fullWindow = true
	buf.w = copy(buf.data, p[k:])
}

// ErrBufferFull indicates that no more data can be buffered.
var ErrBufferFull = errors.New("lz: buffer is full")

// Write writes data into the sequencer. If the Write cannot be completed no
// bytes will be written.
func (buf *RingBuffer) Write(p []byte) (n int, err error) {
	n = buf.available()
	if len(p) > n {
		return 0, ErrBufferFull
	}
	buf.copySlice(p)
	return len(p), nil
}

func (buf *RingBuffer) copyMatch(n int, off int) {
	for n > off {
		buf.copyMatch(off, off)
		n -= off
		if n <= off {
			// no need to double off; prevents also that 2*off < 0
			break
		}
		off *= 2
	}
	// n <= off
	r := buf.w - off
	if r < 0 {
		r += len(buf.data)
	}
	p := buf.data[r:]
	if len(p) < n {
		buf.copySlice(p)
		n -= len(p)
		p = buf.data
	}
	buf.copySlice(p[:n])
}

// WriteMatch writes a match completely or not completely.
func (buf *RingBuffer) WriteMatch(n int, offset int) error {
	if offset <= 0 {
		return fmt.Errorf("lz: offset=%d; must be > 0", offset)
	}
	if k := buf.len(); offset > k {
		return fmt.Errorf("lz: offset=%d; should be <= window (%d)",
			offset, k)
	}
	a := buf.available()
	if n > a {
		return ErrBufferFull
	}
	buf.copyMatch(n, offset)
	return nil
}

// writeSeq writes the sequence to the buffer.
func (buf *RingBuffer) writeSeq(s Seq, literals []byte) (l int, err error) {
	if int64(s.LitLen) > int64(len(literals)) {
		return 0, errors.New("lz: too few literals for serquence")
	}
	if s.Len() > int64(buf.available()) {
		return 0, ErrBufferFull
	}
	if s.Offset == 0 {
		return 0, errors.New("lz: sequence offset must be > 0")
	}
	n := int64(buf.len())
	n += int64(s.LitLen)
	winSize := int64(len(buf.data)) - 1
	if n > winSize {
		n = winSize
	}
	if int64(s.Offset) > n {
		return 0, errors.New("lz: offset too large")
	}
	l, err = buf.Write(literals[:s.LitLen])
	if err != nil {
		return l, err
	}
	buf.copyMatch(int(s.MatchLen), int(s.Offset))
	return l, nil
}

// WriteBlock writes a whole list of sequences, each sequence will be written
// atomically. The functions returns the number of sequences k written, the
// number of literals l consumed and the number of bytes n generated.
func (buf *RingBuffer) WriteBlock(blk *Block) (k, l int, n int64, err error) {
	var s Seq
	for k, s = range blk.Sequences {
		m, err := buf.writeSeq(s, blk.Literals[l:])
		l += m
		n += int64(m)
		if err != nil {
			return k, l, n, err
		}
		n += int64(s.MatchLen)
	}
	k = len(blk.Sequences)
	m, err := buf.Write(blk.Literals[l:])
	l += m
	n += int64(m)
	return k, l, n, err
}

// A RingDecoder decodes sequences and writes data into the writer.
type RingDecoder struct {
	buf RingBuffer
	w   io.Writer
}

// NewDecoder allocates and initializes a decoder. If the windowSize is
// not positive an error will be returned.
func NewDecoder(w io.Writer, windowSize int) (*RingDecoder, error) {
	d := new(RingDecoder)
	err := d.Init(w, windowSize)
	return d, err
}

// Init initializes the decoder. Internal bufferes will be reused if they are
// largen enougn.
func (d *RingDecoder) Init(w io.Writer, windowSize int) error {
	if err := d.buf.Init(windowSize); err != nil {
		return err
	}
	d.w = w
	return nil
}

// Flush writes all decoded data to the underlying writer.
func (d *RingDecoder) Flush() error {
	_, err := d.buf.WriteTo(d.w)
	return err
}

// Write writes data directoly into the decoder.
func (d *RingDecoder) Write(p []byte) (n int, err error) {
	n, err = d.buf.Write(p)
	if err != ErrBufferFull {
		return n, err
	}
	p = p[n:]

	if err = d.Flush(); err != nil {
		return n, err
	}

	var k int
	k, err = d.buf.Write(p)
	n += k
	return n, err
}

// WriteMatch writes a single match into the decoder.
func (d *RingDecoder) WriteMatch(n int, offset int) error {
	err := d.buf.WriteMatch(n, offset)
	if err != ErrBufferFull {
		return err
	}

	if err = d.Flush(); err != nil {
		return err
	}

	return d.buf.WriteMatch(n, offset)
}

// WriteBlock writes a complete block into the decoder.
func (d *RingDecoder) WriteBlock(blk *Block) (k, l int, n int64, err error) {
	k, l, n, err = d.buf.WriteBlock(blk)
	if err != ErrBufferFull {
		return k, l, n, err
	}

	if err = d.Flush(); err != nil {
		return k, l, n, err
	}

	blk.Sequences = blk.Sequences[k:]
	blk.Literals = blk.Literals[l:]
	k2, l2, n2, err := d.buf.WriteBlock(blk)
	k += k2
	l += l2
	n += n2
	return k, l, n, err
}
