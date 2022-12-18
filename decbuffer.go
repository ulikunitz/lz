package lz

import (
	"fmt"
	"io"
)

// DecBuffer provides a simple buffer to decode sequences. The max field gives a
// target that can be exceeded once.
type DecBuffer struct {
	data []byte

	start int64

	r int

	windowSize int
	max        int
}

// Init initialized the buffer. The window size must be larger than 1 and max
// must be larger then the windowSize.
func (buf *DecBuffer) Init(windowSize, max int) error {
	if windowSize < 1 {
		return fmt.Errorf("lz: windowSize must be >1")
	}
	if max <= windowSize {
		return fmt.Errorf("lz: max must be > windowSize")
	}

	if cap(buf.data) < max {
		buf.data = make([]byte, 0, max)
	}
	*buf = DecBuffer{
		data:       buf.data[:0],
		windowSize: windowSize,
		max:        max,
	}

	return nil
}

// Pos returns the file position of the window head.
func (buf *DecBuffer) Pos() int64 {
	return buf.start + int64(len(buf.data))
}

// ByteAtEnd reads the byte with offset i from the end. If it it points outside
// the window the value returned is 0.
func (buf *DecBuffer) ByteAtEnd(i int) byte {
	if !(0 < i && i <= buf.winLen()) {
		return 0
	}
	return buf.data[len(buf.data)-i]
}

// Reset puts the buffer into its initial state.
func (buf *DecBuffer) Reset() {
	buf.start = 0
	buf.data = buf.data[:0]
	buf.r = 0
}

func (buf *DecBuffer) available() int { return buf.max - len(buf.data) }

func (buf *DecBuffer) shrinkable() int {
	r := len(buf.data) - buf.windowSize
	if buf.r < r {
		r = buf.r
	}
	if r < 0 {
		r = 0
	}
	return r
}

// Available provides the amount of data that can be written into the buffer.
func (buf *DecBuffer) Available() int { return buf.shrinkable() + buf.available() }

// Len returns the number of bytes in the unread portion of the buffer.
func (buf *DecBuffer) Len() int { return len(buf.data) - buf.r }

func (buf *DecBuffer) winLen() int {
	n := len(buf.data)
	if n > buf.windowSize {
		n = buf.windowSize
	}
	return n
}

// Read reads data from the buffer. The function never returns an error.
func (buf *DecBuffer) Read(p []byte) (n int, err error) {
	n = copy(p, buf.data[buf.r:])
	buf.r += n
	return n, nil
}

// WriteTo writes all data to read into the writer. It only returns an error if
// the Write fails.
func (buf *DecBuffer) WriteTo(w io.Writer) (n int64, err error) {
	p := buf.data[buf.r:]
	k, err := w.Write(p)
	buf.r += k
	return int64(k), err
}

// shrink moves the window to the front of the buffer if n bytes will be made
// available. Otherwise ErrFullBuffer will be returned.
func (buf *DecBuffer) shrink(n int64) error {
	r := buf.shrinkable()
	if int64(buf.available())+int64(r) < n {
		return ErrFullBuffer
	}
	if r <= 0 {
		return nil
	}
	buf.start += int64(r)
	k := copy(buf.data, buf.data[r:])
	buf.data = buf.data[:k]
	buf.r -= r
	return nil
}

// Write writes the provided byte slice into the buffer and extends the window
// accordingly.
func (buf *DecBuffer) Write(p []byte) (n int, err error) {
	if buf.available() < len(p) {
		if err = buf.shrink(int64(len(p))); err != nil {
			return 0, err
		}
	}
	buf.data = append(buf.data, p...)
	return len(p), err
}

// WriteByte writes a single byte to the buffer and extends the window.
func (buf *DecBuffer) WriteByte(c byte) error {
	if buf.available() < 1 {
		if err := buf.shrink(int64(1)); err != nil {
			return err
		}
	}
	buf.data = append(buf.data, c)
	return nil
}

func (buf *DecBuffer) copyMatch(n, off int) {
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

// WriteMatch writes a match into the buffer and extends the window by the
// match.
func (buf *DecBuffer) WriteMatch(n, offset int) error {
	if offset <= 0 {
		return fmt.Errorf("lz: offset=%d; must be > 0", offset)
	}
	if n < 0 {
		return fmt.Errorf("lz: n=%d; must be > 0", n)
	}
	if n > buf.available() {
		if err := buf.shrink(int64(n)); err != nil {
			return err
		}
	}
	if k := buf.winLen(); offset > k {
		return fmt.Errorf("lz: offset=%d; should be <= window (%d)",
			offset, k)
	}
	buf.copyMatch(n, offset)
	return nil
}

// WriteBlock writes a whole list of sequences, each sequence will be written
// atomically. The functions returns the number of sequences k written, the
// number of literals l consumed and the number of bytes n generated.
func (buf *DecBuffer) WriteBlock(blk Block) (k, l, n int, err error) {
	a := len(buf.data)
	ll := len(blk.Literals)
	var s Seq
	for k, s = range blk.Sequences {
		if int64(s.LitLen) > int64(len(blk.Literals)) {
			n = len(buf.data) - a
			l = ll - len(blk.Literals)
			return k, l, n, fmt.Errorf(
				"lz: LitLen=%d too large; must <=%d",
				s.LitLen, len(blk.Literals))
		}
		winSize := len(buf.data) + int(s.LitLen)
		if winSize > buf.windowSize {
			winSize = buf.windowSize
		}
		off := int(s.Offset)
		if off == 0 && s.MatchLen > 0 {
			l = ll - len(blk.Literals)
			n = len(buf.data) - a
			return k, l, n, fmt.Errorf("off must be > 0")

		}
		if off > winSize {
			l = ll - len(blk.Literals)
			n = len(buf.data) - a
			return k, l, n, fmt.Errorf("off must be <= window size")
		}
		_len := s.Len()
		if _len > int64(buf.available()) {
			if _len > int64(buf.windowSize) {
				l = ll - len(blk.Literals)
				n = len(buf.data) - a
				return k, l, n, fmt.Errorf(
					"seq length > windowSize")
			}
			if err = buf.shrink(_len); err != nil {
				l = ll - len(blk.Literals)
				n = len(buf.data) - a
				return k, l, n, ErrFullBuffer
			}
		}
		buf.data = append(buf.data, blk.Literals[:s.LitLen]...)
		blk.Literals = blk.Literals[s.LitLen:]
		m := int(s.MatchLen)
		for m > off {
			buf.data = append(buf.data,
				buf.data[len(buf.data)-off:]...)
			m -= off
			if m <= off {
				break
			}
			off *= 2
		}
		// m <= off
		d := len(buf.data) - off
		buf.data = append(buf.data, buf.data[d:d+m]...)
	}
	buf.data = append(buf.data, blk.Literals...)
	n = len(buf.data) - a
	return len(blk.Sequences), ll, n, nil
}

// DConfig contains the configuration for a simple Decoder. It provides the
// window size and the MaxSize of the buffer.
type DConfig struct {
	WindowSize int
	MaxSize    int
}

// Verify checks the configuration and returns any errors.
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

// ApplyDefaults applies the defaults for the configuration.
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
	buf DecBuffer
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

// Reset resets the decoder to its initial state.
func (d *Decoder) Reset(w io.Writer) {
	d.buf.Reset()
	d.w = w
}

// Flush writes all decoded data to the underlying writer.
func (d *Decoder) Flush() error {
	_, err := d.buf.WriteTo(d.w)
	return err
}

// Write writes data directoly into the decoder.
func (d *Decoder) Write(p []byte) (n int, err error) {
	n, err = d.buf.Write(p)
	if err != ErrFullBuffer {
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
	if err != ErrFullBuffer {
		return err
	}

	if err = d.Flush(); err != nil {
		return err
	}

	return d.buf.WriteMatch(n, offset)
}

// WriteBlock writes a complete block into the decoder.
func (d *Decoder) WriteBlock(blk Block) (k, l, n int, err error) {
	k, l, n, err = d.buf.WriteBlock(blk)
	if err != ErrFullBuffer {
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
