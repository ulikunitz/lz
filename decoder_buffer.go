// SPDX-FileCopyrightText: © 2021 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

package lz

import (
	"errors"
	"fmt"
	"io"
	"math"
)

// DecoderConfig contains the parameters for the DecoderBuffer and decoder
// types. WindowSize must be smaller than BufferSize. It is recommended to set
// BufferSize to twice the WindowSize.
type DecoderConfig struct {
	// Size of the sliding dictionary window in bytes.
	WindowSize int
	// Maximum size of the buffer in bytes.
	BufferSize int
}

// SetDefaults assigns default values to zero fields in DecoderConfig.
func (cfg *DecoderConfig) SetDefaults() {
	if cfg.WindowSize == 0 {
		cfg.WindowSize = 8 * miB
	}
	if cfg.BufferSize == 0 {
		cfg.BufferSize = 2 * cfg.WindowSize
	}
}

// Verify checks the parameters of the DecoderConfig value and returns an error
// for the first issue found.
func (cfg *DecoderConfig) Verify() error {
	if !(1 <= cfg.BufferSize && int64(cfg.BufferSize) <= math.MaxUint32) {
		return fmt.Errorf(
			"lz.DecConfig: BufferSize=%d out of range [%d..%d]",
			cfg.BufferSize, 1, int64(math.MaxUint32))
	}
	if !(0 <= cfg.WindowSize && cfg.WindowSize < cfg.BufferSize) {
		return fmt.Errorf(
			"lz.DecConfig: WindowSize=%d out of range [%d..BufferSize=%d)",
			cfg.WindowSize, 0, cfg.BufferSize)
	}
	return nil
}

// DecoderBuffer provides a simple buffer for decoding LZ77 sequences. Data is
// the actual buffer. The end of the slice is also the head of the dictionary
// window. R tracks the read position in the buffer and must be less than or
// equal to the length of the Data slice. Off records the total offset and marks
// the end of the Data slice, which is also the end of the dictionary window.
// DecoderConfig provides the configuration parameters WindowSize and
// BufferSize.
type DecoderBuffer struct {
	// Data is the actual buffer. The end of the slice is also the head of
	// the dictionary window.
	Data []byte
	// R tracks the position of the reads from the buffer and must be less
	// or equal to the length of the Data slice.
	R int
	// Off records the total offset and marks the end of the Data slice,
	// which is also the end of the dictionary window.
	Off int64

	// DecConfig provides the configuration parameters WindowSize and
	// BufferSize.
	DecoderConfig
}

// Init initializes the DecoderBuffer.
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

// Reset returns the DecoderBuffer to its initialized state.
func (b *DecoderBuffer) Reset() {
	*b = DecoderBuffer{
		Data:          b.Data[:0],
		DecoderConfig: b.DecoderConfig,
	}
	if cap(b.Data) > b.BufferSize {
		// The default BufferSize is twice the WindowSize.
		b.BufferSize = cap(b.Data)
	}
}

// ByteAtEnd returns the byte at the end of the buffer.
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

// shrink shifts data in the buffer and returns the additional space made
// available, in bytes. It returns 0 if no more space can be provided.
//
// This method is private and is called automatically by various write methods.
func (b *DecoderBuffer) shrink(g int) int {
	delta0 := 0
	if b.BufferSize < cap(b.Data) {
		delta0 = cap(b.Data) - b.BufferSize
		b.BufferSize = cap(b.Data)
		if g <= b.BufferSize {
			return delta0
		}
	}
	delta := doz(len(b.Data), b.WindowSize)
	if b.R < delta {
		delta = b.R
	}
	if delta == 0 {
		return delta0
	}
	k := copy(b.Data, b.Data[delta:])
	b.Data = b.Data[:k]
	b.R -= delta
	return delta0 + delta
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

// Write inserts the slice into the buffer. The method will write the entire
// slice or return 0 and ErrFullBuffer.
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

// WriteMatch appends the ma tch to the end of the buffer. The match will be
// written completely, or n=0 and ErrFullBuffer will be returned.
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

// WriteBlock writes sequences from the block into the buffer. Each sequence is
// written atomically, as the block value is not modified. If there is not
// enough space in the buffer, ErrFullBuffer will be returned.
//
// The growth of the array is not limited to BufferSize. This may consume more
// memory, but increases speed.
//
// The return values n, k, and l indicate the number of bytes written to the
// buffer, the number of sequences, and the number of literals, respectively.
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
