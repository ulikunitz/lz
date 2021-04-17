package lz

import (
	"errors"
	"fmt"
	"io"
)

type Buffer struct {
	data []byte

	r int

	windowSize int
	max        int
}

func (buf *Buffer) Init(windowSize, max int) error {
	if windowSize < 1 {
		return fmt.Errorf("lz: windowSize must be >1")
	}
	if max <= windowSize {
		return fmt.Errorf("lz: max must be > windowSize")
	}

	*buf = Buffer{
		data:       buf.data[:0],
		windowSize: windowSize,
		max:        max,
	}

	return nil
}

func (buf *Buffer) Reset() {
	buf.data = buf.data[:0]
	buf.r = 0
}

func (buf *Buffer) available() int { return buf.max - len(buf.data) }

func (buf *Buffer) len() int {
	n := len(buf.data)
	if n > buf.windowSize {
		n = buf.windowSize
	}
	return n
}

func (buf *Buffer) Read(p []byte) (n int, err error) {
	n = copy(p, buf.data[buf.r:])
	buf.r += n
	return n, nil
}

func (buf *Buffer) WriteTo(w io.Writer) (n int64, err error) {
	k, err := w.Write(buf.data[buf.r:])
	buf.r += k
	return int64(k), err
}

func (buf *Buffer) shrink(n int) error {
	r := len(buf.data) - buf.windowSize
	if r < 0 {
		r = 0
	}
	if buf.r < r {
		r = buf.r
	}
	if buf.available() < n-r {
		return ErrBufferFull
	}
	if r <= 0 {
		return nil
	}
	k := copy(buf.data, buf.data[r:])
	buf.data = buf.data[:k]
	buf.r -= r
	return nil
}

func (buf *Buffer) Write(p []byte) (n int, err error) {
	if buf.available() < len(p) {
		if err = buf.shrink(len(p)); err != nil {
			return 0, err
		}
	}
	buf.data = append(buf.data, p...)
	return len(p), nil
}

func (buf *Buffer) copyMatch(n, off int) {
	for n > off {
		buf.data = append(buf.data,
			buf.data[len(buf.data)-off:]...)
		n -= off
		if n <= off {
			break
		}
		off *= 2
	}
	// n <= off
	k := len(buf.data) - off
	buf.data = append(buf.data, buf.data[k:k+n]...)
}

func (buf *Buffer) WriteMatch(n, offset int) error {
	if n > buf.available() {
		if err := buf.shrink(n); err != nil {
			return err
		}
	}
	if offset <= 0 {
		return fmt.Errorf("lz: offset=%d; must be > 0", offset)
	}
	if n < 0 {
		return fmt.Errorf("lz: n=%d; must be >= 0", n)
	}
	if k := buf.len(); offset > k {
		return fmt.Errorf("lz: offset=%d; should be <= window (%d)",
			offset, k)
	}
	buf.copyMatch(n, offset)
	return nil
}

// writeSeq writes the sequence to the buffer.
func (buf *Buffer) writeSeq(s Seq, literals []byte) (l int, err error) {
	if int64(s.LitLen) > int64(len(literals)) {
		return 0, errors.New("lz: too few literals for sequence")
	}
	if n := s.Len(); n > int64(buf.available()) {
		k := int(n)
		if k < 0 {
			return 0, ErrBufferFull
		}
		if err = buf.shrink(k); err != nil {
			return 0, err
		}
	}
	if s.Offset == 0 {
		return 0, errors.New("lz: sequence offset must be > 0")
	}
	w := int64(buf.len())
	w += int64(s.LitLen)
	if w > int64(buf.windowSize) {
		w = int64(buf.windowSize)
	}
	if int64(s.Offset) > w {
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
func (buf *Buffer) WriteBlock(blk *Block) (k, l int, n int64, err error) {
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

type DConfig struct {
	WindowSize int
	MaxSize    int
}

func (cfg *DConfig) Verify() error {
	if cfg.WindowSize < 1 {
		return fmt.Errorf("lz: windowSize=%d must be >=1",
			cfg.WindowSize)
	}
	if cfg.MaxSize <= cfg.WindowSize {
		return fmt.Errorf("lz: MaxSize=%d must be > WindowSize=%d",
			cfg.MaxSize, cfg.WindowSize)
	}
	return nil
}

func (cfg *DConfig) ApplyDefaults() {
	if cfg.WindowSize == 0 {
		cfg.WindowSize = 8 * 1024 * 1024
	}
	if cfg.MaxSize == 0 {
		cfg.MaxSize = 2 * cfg.WindowSize
	}
}

// A Decoder decodes sequences and writes data into the writer.
type Decoder struct {
	buf Buffer
	w   io.Writer
}

// NewDecoder allocates and initializes a decoder. If the windowSize is
// not positive an error will be returned.
func NewDecoder(w io.Writer, cfg DConfig) (*Decoder, error) {
	d := new(Decoder)
	err := d.Init(w, cfg)
	return d, err
}

// Init initializes the decoder. Internal bufferes will be reused if they are
// largen enougn.
func (d *Decoder) Init(w io.Writer, cfg DConfig) error {
	cfg.ApplyDefaults()
	if err := cfg.Verify(); err != nil {
		return err
	}
	if err := d.buf.Init(cfg.WindowSize, cfg.MaxSize); err != nil {
		return err
	}
	d.w = w
	return nil
}

// Flush writes all decoded data to the underlying writer.
func (d *Decoder) Flush() error {
	_, err := d.buf.WriteTo(d.w)
	return err
}

// Write writes data directoly into the decoder.
func (d *Decoder) Write(p []byte) (n int, err error) {
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
func (d *Decoder) WriteMatch(n int, offset int) error {
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
func (d *Decoder) WriteBlock(blk *Block) (k, l int, n int64, err error) {
	k, l, n, err = d.buf.WriteBlock(blk)
	if err != ErrBufferFull {
		return k, l, n, err
	}

	if err = d.Flush(); err != nil {
		return k, l, n, err
	}

	var copy Block
	copy.Sequences = blk.Sequences[k:]
	copy.Literals = blk.Literals[l:]
	k2, l2, n2, err := d.buf.WriteBlock(&copy)
	k += k2
	l += l2
	n += n2
	return k, l, n, err
}
