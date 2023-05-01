package lz

import (
	"errors"
	"fmt"
	"io"
)

// DecoderConfig contains the parameters for the [DecBuffer] and [Decoder] types.
// The WindowSize must be smaller than the BufferSize. It is recommended to set
// the BufferSize twice as large as the WindowSize.
type DecoderConfig struct {
	// Size of the sliding dictionary window in bytes.
	WindowSize int
	// Maximum size of the buffer in bytes.
	BufferSize int
}

// SetDefaults sets the zero values in DecConfig to default values. Note that
// the default BufferSize is twice the WindowSize.
func (cfg *DecoderConfig) SetDefaults() {
	if cfg.WindowSize == 0 {
		cfg.WindowSize = 8 * miB
	}
	if cfg.BufferSize == 0 {
		cfg.BufferSize = 2 * cfg.WindowSize
	}
}

// Verify checks the parameters of the [DecConfig] value and returns an error
// for the first problem.
func (cfg *DecoderConfig) Verify() error {
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

// DecoderBuffer provides a simple buffer for the decoding of LZ77 sequences.
type DecoderBuffer struct {
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
	DecoderConfig
}

// Init initializes the [DecBuffer] value.
func (b *DecoderBuffer) Init(cfg DecoderConfig) error {
	cfg.SetDefaults()
	var err error
	if err = cfg.Verify(); err != nil {
		return err
	}
	*b = DecoderBuffer{
		Data:          b.Data[:0],
		DecoderConfig: cfg,
	}
	if cap(b.Data) > b.BufferSize {
		b.BufferSize = cap(b.Data)
	}
	return nil
}

// Reset puts the DecBuffer back to the initialized status.
func (b *DecoderBuffer) Reset() {
	*b = DecoderBuffer{
		Data:          b.Data[:0],
		DecoderConfig: b.DecoderConfig,
	}
	if cap(b.Data) > b.BufferSize {
		b.BufferSize = cap(b.Data)
	}
}

// ByteAtEnd returns byte at end of the buffer
func (b *DecoderBuffer) ByteAtEnd(off int) byte {
	i := len(b.Data) - off
	if !(0 <= i && i < len(b.Data)) {
		return 0
	}
	return b.Data[i]
}

// Read reads decoded data from the buffer.
func (b *DecoderBuffer) Read(p []byte) (n int, err error) {
	n = copy(p, b.Data[b.R:])
	b.R += n
	return n, nil
}

// WriteTo writes the decoded data to the writer.
func (b *DecoderBuffer) WriteTo(w io.Writer) (n int64, err error) {
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
func (b *DecoderBuffer) shrink(g int) int {
	if b.BufferSize < cap(b.Data) {
		b.BufferSize = cap(b.Data)
		if g <= b.BufferSize {
			return 0
		}
	}
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

// WriteByte writes a single byte into the buffer.
func (b *DecoderBuffer) WriteByte(c byte) error {
	g := len(b.Data) + 1
	if g > b.BufferSize {
		if g -= b.shrink(g); g > b.BufferSize {
			return ErrFullBuffer
		}
	}
	b.Data = append(b.Data, c)
	b.Off++
	return nil
}

// Write puts the slice into the buffer. The method will write the slice only
// fully or will return 0, [ErrFullBuffer].
func (b *DecoderBuffer) Write(p []byte) (n int, err error) {
	n = len(p)
	g := len(b.Data) + n
	if g > b.BufferSize {
		if g -= b.shrink(g); g > b.BufferSize {
			return 0, ErrFullBuffer
		}
	}
	b.Data = append(b.Data, p...)
	b.Off += int64(n)
	return n, nil
}

// WriteMatch puts the match at the end of the buffer. The match will only be
// written completely or n=0 and [ErrFullBuffer] will be returned.
func (b *DecoderBuffer) WriteMatch(m, o uint32) (n int, err error) {
	if o == 0 && m > 0 {
		return 0, errOffset
	}
	winLen := len(b.Data)
	if winLen > b.WindowSize {
		winLen = b.WindowSize
	}
	if int64(o) > int64(winLen) {
		return 0, errOffset
	}
	_m := int64(m)
	if a := b.BufferSize - len(b.Data); _m > int64(a) {
		if _m > int64(b.WindowSize) {
			return 0, errMatchLen
		}
		if a += b.shrink(int(_m) + len(b.Data)); _m > int64(a) {
			return 0, ErrFullBuffer
		}
	}
	n = int(_m)
	off := int(o)
	for n > off {
		b.Data = append(b.Data, b.Data[len(b.Data)-off:]...)
		n -= off
		if n <= off {
			break
		}
		off <<= 1
	}
	// n <= off
	j := len(b.Data) - off
	b.Data = append(b.Data, b.Data[j:j+n]...)
	b.Off += _m
	return int(_m), nil
}

var (
	errLitLen   = errors.New("lz: LitLen out of range")
	errMatchLen = errors.New("lz: MatchLen out of range")
	errOffset   = errors.New("lz: Offset out of range")
)

// WriteBlock writes sequences from the block into the buffer. A single sequence
// will be written in an atomic manner, because the block value will not be
// modified. If there is not enough space in the buffer [ErrFullBuffer] will be
// returned.
//
// We are not limiting the growth of the array to BufferSize. We may consume
// more memory but we are faster.
//
// The return values n, k and l provide the number of bytes written into the
// buffer, the number of sequences as well as the number of literals.
func (b *DecoderBuffer) WriteBlock(blk Block) (n, k, l int, err error) {
	ld := len(b.Data)
	ll := len(blk.Literals)
	var s Seq
	for k, s = range blk.Sequences {
		if int64(s.LitLen) > int64(len(blk.Literals)) {
			err = errLitLen
			goto end
		}
		if s.Offset == 0 && s.MatchLen > 0 {
			err = errOffset
			goto end
		}
		winLen := len(b.Data) + int(s.LitLen)
		if winLen > b.WindowSize {
			winLen = b.WindowSize
		}
		if int64(s.Offset) > int64(winLen) {
			err = errOffset
			goto end
		}
		g := int64(s.LitLen) + int64(s.MatchLen)
		if a := b.BufferSize - len(b.Data); g > int64(a) {
			if g > int64(b.WindowSize) {
				err = errMatchLen
				goto end
			}
			if a += b.shrink(int(g) + len(b.Data)); g > int64(a) {
				err = ErrFullBuffer
				goto end
			}
		}
		b.Data = append(b.Data, blk.Literals[:s.LitLen]...)
		blk.Literals = blk.Literals[s.LitLen:]
		n := int(s.MatchLen)
		off := int(s.Offset)
		for n > off {
			b.Data = append(b.Data, b.Data[len(b.Data)-off:]...)
			n -= off
			if n <= off {
				break
			}
			off <<= 1
		}
		// n <= off
		j := len(b.Data) - off
		b.Data = append(b.Data, b.Data[j:j+n]...)
	}
	k = len(blk.Sequences)
	{ // block required to allow goto over it.
		g := len(b.Data) + len(blk.Literals)
		if g > b.BufferSize {
			if g -= b.shrink(g); g > b.BufferSize {
				err = ErrFullBuffer
				goto end
			}
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
	buf DecoderBuffer
	w   io.Writer
}

// NewDecoder creates a new decoder. The first issue with the configuration
// will be reported.
func NewDecoder(w io.Writer, cfg DecoderConfig) (*Decoder, error) {
	d := new(Decoder)
	err := d.Init(w, cfg)
	return d, err
}

// Init initializes the decoder. The first issue of the configuration value will
// be reported as error.
func (d *Decoder) Init(w io.Writer, cfg DecoderConfig) error {
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
// bytes, the number k of parsers and the number l of literal bytes written
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
