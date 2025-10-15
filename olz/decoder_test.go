package olz

import "io"

// The following code is only provided for testing purposes.

// decoder decodes LZ77 sequences and writes them into the writer.
type decoder struct {
	buf DecoderBuffer
	w   io.Writer
}

// newDecoder creates a new decoder. The first issue with the configuration
// will be reported.
func newDecoder(w io.Writer, cfg DecoderConfig) (*decoder, error) {
	d := new(decoder)
	err := d.Init(w, cfg)
	return d, err
}

// Init initializes the decoder. The first issue of the configuration value will
// be reported as error.
func (d *decoder) Init(w io.Writer, cfg DecoderConfig) error {
	var err error
	if err = d.buf.Init(cfg); err != nil {
		return err
	}
	d.w = w
	return nil
}

// Reset initializes the decoder with a new io.Writer.
func (d *decoder) Reset(w io.Writer) {
	d.buf.Reset()
	d.w = w
}

// Flush writes all remaining data in the buffer to the underlying writer.
func (d *decoder) Flush() error {
	_, err := d.buf.WriteTo(d.w)
	return err
}

// WriteByte writes a single byte into the decoder.
func (d *decoder) WriteByte(c byte) error {
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
func (d *decoder) Write(p []byte) (n int, err error) {
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
func (d *decoder) WriteBlock(blk Block) (n, k, l int, err error) {
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
