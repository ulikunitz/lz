package lz

import (
	"errors"
	"fmt"
	"io"
)

// DecoderWindow decodes sequences. The decoded data must be read from the
// decoder window and the WriteTo method is the most simple to use for that.
type DecoderWindow struct {
	data []byte

	// reader index; start of valid data
	r int

	// writer index; end of valid data
	w int

	// fullWindow marks the situation where sw.w doesn't contain the window
	// size
	fullWindow bool
}

// Init initializes the decoder window. An existing data slice might be reused.
func (dw *DecoderWindow) Init(windowSize int) error {
	if windowSize < 1 {
		return fmt.Errorf("lz: winSize must be >= 1")
	}
	if cap(dw.data) > windowSize {
		*dw = DecoderWindow{data: dw.data[:windowSize+1]}
	} else {
		*dw = DecoderWindow{data: make([]byte, windowSize+1)}
	}
	return nil
}

// Read reads data from the writer. It will always try to return as much data as
// possible.
func (dw *DecoderWindow) Read(p []byte) (n int, err error) {
	if dw.r > dw.w {
		n = copy(p, dw.data[dw.r:])
		p = p[n:]
		dw.r += n
		if dw.r < len(dw.data) {
			return n, nil
		}
		dw.r = 0
	}
	k := copy(p, dw.data[dw.r:dw.w])
	n += k
	dw.r += k
	return n, nil
}

// WriteTo writes data into the writer as much as it is possible.
func (dw *DecoderWindow) WriteTo(w io.Writer) (n int64, err error) {
	if dw.r > dw.w {
		k, err := w.Write(dw.data[dw.r:])
		dw.r += k
		n = int64(k)
		if err != nil {
			return n, err
		}
		if dw.r != len(dw.data) {
			panic(fmt.Errorf("sw.r=%d; want len(sw.data)=%d", dw.r,
				len(dw.data)))
		}
		dw.r = 0
	}
	k, err := w.Write(dw.data[dw.r:dw.w])
	dw.r += k
	n += int64(k)
	return n, err
}

func (dw *DecoderWindow) available() int {
	n := dw.r - dw.w - 1
	if n < 0 {
		n += len(dw.data)
	}
	return n
}

func (dw *DecoderWindow) len() int {
	if dw.fullWindow {
		return len(dw.data) - 1
	}
	return dw.w
}

func (dw *DecoderWindow) copySlice(p []byte) {
	q := dw.data[dw.w:]
	k := copy(q, p)
	if k < len(q) {
		dw.w += k
		return
	}
	dw.fullWindow = true
	dw.w = copy(dw.data, p[k:])
}

// ErrBufferFull indicates that no more data can be bufferd.
var ErrBufferFull = errors.New("buffer is full")

// Write writes data into the sequencer. If the Write cannot be completed no
// bytes will be written.
func (dw *DecoderWindow) Write(p []byte) (n int, err error) {
	n = dw.available()
	if len(p) > n {
		return 0, ErrBufferFull
	}
	dw.copySlice(p)
	return len(p), nil
}

func (dw *DecoderWindow) copyMatch(n int, off int) {
	for n > off {
		dw.copyMatch(off, off)
		n -= off
		if n <= off {
			// no need to double off; prevents also that 2*off < 0
			break
		}
		off *= 2
	}
	// n <= off
	r := dw.w - off
	if r < 0 {
		r += len(dw.data)
	}
	p := dw.data[r:]
	if len(p) < n {
		dw.copySlice(p)
		n -= len(p)
		p = dw.data
	}
	dw.copySlice(p[:n])
}

// WriteMatch writes a match completely or not completely.
func (dw *DecoderWindow) WriteMatch(n int, offset int) error {
	if offset <= 0 {
		return fmt.Errorf("lz: offset=%d; must be > 0", offset)
	}
	if k := dw.len(); offset > k {
		return fmt.Errorf("lz: offset=%d; should be <= window (%d)",
			offset, k)
	}
	a := dw.available()
	if n > a {
		return ErrBufferFull
	}
	dw.copyMatch(n, offset)
	return nil
}

// writeSeq writes the sequence to the buffer.
func (dw *DecoderWindow) writeSeq(s Seq, literals []byte) (l int, err error) {
	if int64(s.LitLen) > int64(len(literals)) {
		return 0, errors.New("lz: too few literals for serquence")
	}
	if s.Len() > int64(dw.available()) {
		return 0, ErrBufferFull
	}
	if s.Offset == 0 {
		return 0, errors.New("lz: sequence offset must be > 0")
	}
	n := int64(dw.len())
	n += int64(s.LitLen)
	winSize := int64(len(dw.data)) - 1
	if n > winSize {
		n = winSize
	}
	if int64(s.Offset) > n {
		return 0, errors.New("lz: offset too large")
	}
	l, err = dw.Write(literals[:s.LitLen])
	if err != nil {
		return l, err
	}
	dw.copyMatch(int(s.MatchLen), int(s.Offset))
	return l, nil
}

// WriteBlock writes a whole list of sequences, each sequence will be written
// atomically. The functions returns the number of sequences k written, the
// number of literals l consumed and the number of bytes n generated.
func (dw *DecoderWindow) WriteBlock(blk *Block) (k, l int, n int64, err error) {
	var s Seq
	for k, s = range blk.Sequences {
		m, err := dw.writeSeq(s, blk.Literals[l:])
		l += m
		n += int64(m)
		if err != nil {
			return k, l, n, err
		}
		n += int64(s.MatchLen)
	}
	k = len(blk.Sequences)
	m, err := dw.Write(blk.Literals[l:])
	l += m
	n += int64(m)
	return k, l, n, err
}

// A Decoder decodes sequences and writes data into the writer.
type Decoder struct {
	window DecoderWindow
	w      io.Writer
}

// Init initializes the decoder. Internal bufferes will be reused if they are
// largen enougn.
func (d *Decoder) Init(w io.Writer, windowSize int) error {
	if err := d.window.Init(windowSize); err != nil {
		return err
	}
	d.w = w
	return nil
}

// Flush writes all decoded data to the underlying writer.
func (d *Decoder) Flush() error {
	_, err := d.window.WriteTo(d.w)
	return err
}

// Write writes data directoly into the decoder.
func (d *Decoder) Write(p []byte) (n int, err error) {
	n, err = d.window.Write(p)
	if err != ErrBufferFull {
		return n, err
	}
	p = p[n:]

	if err = d.Flush(); err != nil {
		return n, err
	}

	var k int
	k, err = d.window.Write(p)
	n += k
	return n, err
}

// WriteMatch writes a single match into the decoder.
func (d *Decoder) WriteMatch(n int, offset int) error {
	err := d.window.WriteMatch(n, offset)
	if err != ErrBufferFull {
		return err
	}

	if err = d.Flush(); err != nil {
		return err
	}

	return d.window.WriteMatch(n, offset)
}

// WriteBlock writes a complete block into the decoder.
func (d *Decoder) WriteBlock(blk *Block) (k, l int, n int64, err error) {
	k, l, n, err = d.window.WriteBlock(blk)
	if err != ErrBufferFull {
		return k, l, n, err
	}

	if err = d.Flush(); err != nil {
		return k, l, n, err
	}

	blk.Sequences = blk.Sequences[k:]
	blk.Literals = blk.Literals[l:]
	k2, l2, n2, err := d.window.WriteBlock(blk)
	k += k2
	l += l2
	n += n2
	return k, l, n, err
}
