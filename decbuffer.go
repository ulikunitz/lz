package lz

import (
	"fmt"
	"io"
)

// DecConfig contains the parameters for the [DecBuffer] and [Decoder] types.
// The WindowSize must be smaller than the BufferSize. It is recommended to set
// the BufferSize twice as large as the WindowSize.
type DecConfig struct {
	// Size of the sliding dictionary window in bytes.
	WindowSize int
	// Maximum size of the buffer in bytes.
	BufferSize int
}

// SetDefaults sets the zero values in DecConfig to default values. Note that
// the default BufferSize is twice the WindowSize.
func (cfg *DecConfig) SetDefaults() {
	if cfg.WindowSize == 0 {
		cfg.WindowSize = 8 * miB
	}
	if cfg.BufferSize == 0 {
		cfg.BufferSize = 2 * cfg.WindowSize
	}
}

// Verify checks the parameters of the [DecConfig] value and returns an error
// for the first problem.
func (cfg *DecConfig) Verify() error {
	if !(1 <= cfg.BufferSize && int64(cfg.BufferSize) <= maxUint32) {
		return fmt.Errorf(
			"lz.DecConfig: BufferSize=%d out of range [%d..%d]",
			cfg.BufferSize, 1, int64(maxUint32))
	}
	if !(0 <= cfg.WindowSize && cfg.WindowSize < cfg.BufferSize) {
		return fmt.Errorf(
			"lz.DecConfig: WindowSize=%d out of range [%d..BufferSize=%d)",
			cfg.WindowSize, 0, cfg.BufferSize)
	}
	return nil
}

// DecBuffer provides a simple buffer for the decoding of LZ77 sequences.
type DecBuffer struct {
	// Data is the actual buffer. The end of the slice is also the head of
	// the dictionary window.
	Data []byte
	// R tracks the position of the reads from the buffer and must be less
	// or equal the length of the Data slice.
	R int
	// Off records the total offset and marks the end of the Data slice,
	// which is also the end of the dictionary window.
	Off int64

	// DecConfig provides the configuration parameters WindowSize and
	// BufferSize.
	DecConfig
}

// Init initializes the [DecBuffer] value.
func (b *DecBuffer) Init(cfg DecConfig) error {
	cfg.SetDefaults()
	var err error
	if err = cfg.Verify(); err != nil {
		return err
	}
	*b = DecBuffer{
		Data:      b.Data[:0],
		DecConfig: cfg,
	}
	return nil
}

// Reset puts the DecBuffer back to the initialized status.
func (b *DecBuffer) Reset() {
	*b = DecBuffer{
		Data:      b.Data[:0],
		DecConfig: b.DecConfig,
	}
}

// Read reads decoded data from the buffer.
func (b *DecBuffer) Read(p []byte) (n int, err error) {
	n = copy(p, b.Data[b.R:])
	b.R += n
	return n, nil
}

// WriteTo writes the decoded data to the writer.
func (b *DecBuffer) WriteTo(w io.Writer) (n int64, err error) {
	k, err := w.Write(b.Data[b.R:])
	b.R += k
	return int64(k), err
}

// shrink shifts data in the buffer and returns the additional space in bytes
// that has been made available. Note that shrink will return 0 if it cannot
// provide more space.
//
// The method is private because it is called by the various write methods
// automatically.
func (b *DecBuffer) shrink() int {
	delta := doz(len(b.Data), b.WindowSize)
	if b.R < delta {
		delta = b.R
	}
	if delta == 0 {
		return 0
	}
	k := copy(b.Data, b.Data[delta:])
	b.Data = b.Data[:k]
	b.R -= delta
	return delta
}

func (b *DecBuffer) grow(g int) {
	if g <= cap(b.Data) {
		return
	}
	c := 2 * g
	if 0 <= c && c < 1024 {
		c = 1024
	} else {
		c = ((c + 1<<10 - 1) >> 10) << 10
	}
	if c >= b.BufferSize || c < 0 {
		c = b.BufferSize
	}
	// Allocate the buffer.
	p := b.Data
	b.Data = make([]byte, len(b.Data), c)
	copy(b.Data, p)
}

// WriteByte writes a single byte into the buffer.
func (b *DecBuffer) WriteByte(c byte) error {
	g := len(b.Data) + 1
	if g > b.BufferSize {
		if g -= b.shrink(); g > b.BufferSize {
			return ErrFullBuffer
		}
	}
	if g > cap(b.Data) {
		b.grow(g)
	}
	b.Data = append(b.Data, c)
	b.Off++
	return nil
}

// Write puts the slice into the buffer. The method will write the slice only
// fully or will return 0, [ErrFullBuffer].
func (b *DecBuffer) Write(p []byte) (n int, err error) {
	n = len(p)
	g := len(b.Data) + n
	if g > b.BufferSize {
		if g -= b.shrink(); g > b.BufferSize {
			return 0, ErrFullBuffer
		}
	}
	if g > cap(b.Data) {
		b.grow(g)
	}

	b.Data = append(b.Data, p...)
	b.Off += int64(n)
	return n, nil
}

// WriteMatch puts the match at the end of the buffer. The match will only be
// written completely or n=0 and [ErrFullBuffer] will be returned.
func (b *DecBuffer) WriteMatch(m, o uint32) (n int, err error) {
	if !(1 <= o && int64(o) <= int64(b.WindowSize)) {
		return 0, fmt.Errorf(
			"lz.DecBuffer.WriteMatch: o=%d is outside range [%d..b.WindowSize=%d]",
			o, 1, b.WindowSize)
	}
	if int64(m) > int64(b.BufferSize) {
		return 0, fmt.Errorf(
			"lz.DecBuffer.WriterMatch: m=%d is larger than BufferSize=%d",
			m, b.BufferSize)
	}
	n = int(m)
	g := len(b.Data) + n
	if g > b.BufferSize {
		if g -= b.shrink(); g > b.BufferSize {
			return 0, ErrFullBuffer
		}
	}
	if g > cap(b.Data) {
		b.grow(g)
	}
	off := int(o)
	j := len(b.Data) - off
	for n > off {
		b.Data = append(b.Data, b.Data[j:]...)
		n -= off
		off <<= 1
	}
	// n <= off
	b.Data = append(b.Data, b.Data[j:j+n]...)
	b.Off += int64(m)
	return int(m), nil
}

// WriteBlock writes sequences from the block into the buffer. A single sequence
// will be written in an atomic manner, because the block value will not be
// modified. If there is not enough space in the buffer [ErrFullBuffer] will be
// returned.
//
// The return values n, k and l provide the number of bytes written into the
// buffer, the number of sequences as well as the number of literals.
func (b *DecBuffer) WriteBlock(blk Block) (n, k, l int, err error) {
	ld := len(b.Data)
	ll := len(blk.Literals)
	var s Seq
	for k, s = range blk.Sequences {
		if int64(s.LitLen) > int64(len(blk.Literals)) {
			err = fmt.Errorf(
				"lz: LitLen=%d > len(blk.Literals)=%d",
				s.LitLen, len(blk.Literals))
			goto end
		}
		winLen := min(len(b.Data)+int(s.LitLen), b.WindowSize)
		if !(1 <= s.Offset && int64(s.Offset) <= int64(winLen)) {
			err = fmt.Errorf(
				"lz: Offset=%d is outside range [%d..%d]",
				s.Offset, 1, winLen)
			goto end
		}
		n = int(s.LitLen + s.MatchLen)
		if !(0 <= n && n <= b.BufferSize) {
			err = fmt.Errorf(
				"lz.DecBuffer: length  of sequence %+v is out of range [%d..%d]",
				s, 0, b.BufferSize)
			goto end
		}
		g := len(b.Data) + n
		if g > b.BufferSize {
			if g -= b.shrink(); g > b.BufferSize {
				err = ErrFullBuffer
				goto end
			}
		}
		if g > cap(b.Data) {
			b.grow(g)
		}
		b.Data = append(b.Data, blk.Literals[:s.LitLen]...)
		blk.Literals = blk.Literals[s.LitLen:]
		n = int(s.MatchLen)
		off := int(s.Offset)
		j := len(b.Data) - off
		for n > off {
			b.Data = append(b.Data, b.Data[j:]...)
			n -= off
			off <<= 1
		}
		// n <= off
		b.Data = append(b.Data, b.Data[j:j+n]...)
	}
	k = len(blk.Sequences)
	{ // block required to allow goto over it.
		g := len(b.Data) + len(blk.Literals)
		if g > b.BufferSize {
			if g -= b.shrink(); g > b.BufferSize {
				err = ErrFullBuffer
				goto end
			}
		}
		if g > cap(b.Data) {
			b.grow(g)
		}
	}
	b.Data = append(b.Data, blk.Literals...)
	blk.Literals = blk.Literals[:0]
end:
	n = len(b.Data) - ld
	b.Off += int64(n)
	l = ll - len(blk.Literals)
	return n, k, l, err
}

// Decoder decodes LZ77 sequences and writes them into the writer.
type Decoder struct {
	buf DecBuffer
	w   io.Writer
}

// NewDecoder creates a new decoder. The first issue with the configuration
// will be reported.
func NewDecoder(w io.Writer, cfg DecConfig) (*Decoder, error) {
	d := new(Decoder)
	err := d.Init(w, cfg)
	return d, err
}

// Init initializes the decoder. The first issue of the configuration value will
// be reported as error.
func (d *Decoder) Init(w io.Writer, cfg DecConfig) error {
	var err error
	if err = d.buf.Init(cfg); err != nil {
		return err
	}
	d.w = w
	return nil
}

// Reset initializes the decoder with a new io.Writer.
func (d *Decoder) Reset(w io.Writer) {
	d.buf.Reset()
	d.w = w
}

// Flush writes all remaining data in the buffer to the underlying writer.
func (d *Decoder) Flush() error {
	_, err := d.buf.WriteTo(d.w)
	return err
}

// WriteByte writes a single byte into the decoder.
func (d *Decoder) WriteByte(c byte) error {
	var err error
	for {
		err = d.buf.WriteByte(c)
		if err != ErrFullBuffer {
			return err
		}
		_, err = d.buf.WriteTo(d.w)
		if err != nil {
			return err
		}
	}
}

// Write writes the slice into the buffer.
func (d *Decoder) Write(p []byte) (n int, err error) {
	for {
		k, err := d.buf.Write(p)
		n += k
		if err != ErrFullBuffer {
			return n, err
		}
		_, err = d.buf.WriteTo(d.w)
		if err != nil {
			return n, err
		}
		p = p[k:]
	}
}

// WriteBlock writes the block into the decoder. It returns the number n of
// bytes, the number k of sequencers and the number l of literal bytes written
// to the decoder.
func (d *Decoder) WriteBlock(blk Block) (n, k, l int, err error) {
	for {
		nn, kk, ll, err := d.buf.WriteBlock(blk)
		n += nn
		k += kk
		l += ll
		if err != ErrFullBuffer {
			return n, k, l, err
		}
		_, err = d.buf.WriteTo(d.w)
		if err != nil {
			return n, k, l, err
		}
		blk.Sequences = blk.Sequences[kk:]
		blk.Literals = blk.Literals[ll:]
	}
}
