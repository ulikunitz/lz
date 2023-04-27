package lz

import (
	"fmt"
	"io"
)

type DecConfig struct {
	WindowSize int
	BufferSize int
}

func (cfg *DecConfig) SetDefaults() {
	if cfg.WindowSize == 0 {
		cfg.WindowSize = 8 * miB
	}
	if cfg.BufferSize == 0 {
		cfg.BufferSize = 2 * cfg.WindowSize
	}
}

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

type DecBuffer struct {
	Data []byte
	R    int
	// Off records the total offset and marks the end of the Data slice,
	// which is also the end of the dictionary window.
	Off int64

	DecConfig
}

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

func (b *DecBuffer) Reset() {
	*b = DecBuffer{
		Data:      b.Data[:0],
		DecConfig: b.DecConfig,
	}
}

func (b *DecBuffer) Read(p []byte) (n int, err error) {
	n = copy(p, b.Data[b.R:])
	b.R += n
	return n, nil
}

func (b *DecBuffer) WriteTo(w io.Writer) (n int64, err error) {
	k, err := w.Write(b.Data[b.R:])
	b.R += k
	return int64(k), err
}

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

func (b *DecBuffer) WriteByte(c byte) error {
	n := b.BufferSize - len(b.Data)
	if n <= 0 {
		n += b.shrink()
		if n <= 0 {
			return ErrFullBuffer
		}
	}
	b.Data = append(b.Data, c)
	b.Off++
	return nil
}

func (b *DecBuffer) Write(p []byte) (n int, err error) {
	n = b.BufferSize - len(b.Data)
	if n < len(p) {
		n += b.shrink()
		if n < len(p) {
			return 0, ErrFullBuffer
		}
	}
	n = len(p)
	b.Data = append(b.Data, p...)
	b.Off += int64(n)
	return n, nil
}

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
	a := b.BufferSize - len(b.Data)
	if int64(a) < int64(m) {
		a += b.shrink()
		if int64(a) < int64(m) {
			return 0, ErrFullBuffer
		}
	}
	n = int(m)
	off := int(o)
	for n > off {
		b.Data = append(b.Data, b.Data[len(b.Data)-off:]...)
		n -= off
		if n <= off {
			break
		}
		off *= 2
	}
	// n <= off
	k := len(b.Data) - off
	b.Data = append(b.Data, b.Data[k:k+n]...)
	b.Off += int64(m)
	return n, nil
}

func (b *DecBuffer) WriteBlock(blk Block) (n, k, l int, err error) {
	ld := len(b.Data)
	ll := len(blk.Literals)
	var (
		s Seq
		m int
	)
	for k, s = range blk.Sequences {
		if int64(s.LitLen) > int64(len(blk.Literals)) {
			err = fmt.Errorf(
				"lz: LitLen=%d > len(blk.Literals)=%d",
				s.LitLen, len(blk.Literals))
			goto end
		}
		if !(1 <= s.Offset && int64(s.Offset) <= int64(b.WindowSize)) {
			err = fmt.Errorf(
				"lz: Offset=%d is outside range [%d..b.WindowSize=%d]",
				s.Offset, 1, b.WindowSize)
			goto end
		}
		if int64(s.MatchLen) > int64(b.BufferSize) {
			err = fmt.Errorf(
				"lz.DecBuffer: MatchLen=%d is larger than BufferSize=%d",
				s.MatchLen, b.BufferSize)
			goto end
		}
		m = int(s.LitLen + s.MatchLen)
		if m < 0 {
			err = fmt.Errorf(
				"lz.DecBuffer: length  of sequence %+v is too high",
				s)
			goto end
		}
		n = b.BufferSize - len(b.Data)
		if n < m {
			n += b.shrink()
			if n < m {
				err = ErrFullBuffer
				goto end
			}
		}
		b.Data = append(b.Data, blk.Literals[:s.LitLen]...)
		blk.Literals = blk.Literals[s.LitLen:]
		m = int(s.MatchLen)
		off := int(s.Offset)
		for m > off {
			b.Data = append(b.Data, b.Data[len(b.Data)-off:]...)
			m -= off
			if m <= off {
				break
			}
			off *= 2
		}
		// m <= off
		d := len(b.Data) - off
		b.Data = append(b.Data, b.Data[d:d+m]...)
	}
	k = len(blk.Sequences)
	m = len(blk.Literals)
	n = b.BufferSize - len(b.Data)
	if n < m {
		n += b.shrink()
		if n < m {
			err = ErrFullBuffer
			goto end
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

type Decoder struct {
	buf DecBuffer
	w   io.Writer
}

func NewDecoder(w io.Writer, cfg DecConfig) (*Decoder, error) {
	d := new(Decoder)
	err := d.Init(w, cfg)
	return d, err
}

func (d *Decoder) Init(w io.Writer, cfg DecConfig) error {
	var err error
	if err = d.buf.Init(cfg); err != nil {
		return err
	}
	d.w = w
	return nil
}

func (d *Decoder) Reset(w io.Writer) {
	d.buf.Reset()
	d.w = w
}

func (d *Decoder) Flush() error {
	_, err := d.buf.WriteTo(d.w)
	return err
}

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
