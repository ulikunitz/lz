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

// DecoderOptions contains the parameters for the DecoderBuffer and decoder
// types. WindowSize must be smaller than BufferSize. It is recommended to set
// BufferSize to twice the WindowSize.
type DecoderOptions struct {
	// Size of the sliding dictionary window in bytes.
	WindowSize int
	// Maximum size of the buffer in bytes.
	BufferSize int
}

// SetDefaults assigns default values to zero fields in DecoderConfig.
func (cfg *DecoderOptions) SetDefaults() {
	if cfg.WindowSize == 0 {
		cfg.WindowSize = 8 << 20
	}
	if cfg.BufferSize == 0 {
		cfg.BufferSize = 2 * cfg.WindowSize
	}
}

// Verify checks the parameters of the DecoderConfig value and returns an error
// for the first issue found.
func (cfg *DecoderOptions) Verify() error {
	if !(0 <= cfg.WindowSize && cfg.WindowSize <= math.MaxUint32) {
		return fmt.Errorf(
			"lz.DecConfig: WindowSize=%d out of range [%d..%d]",
			cfg.WindowSize, 0, math.MaxUint32)
	}
	if !(cfg.WindowSize < cfg.BufferSize) {
		return fmt.Errorf(
			"lz.DecConfig: BufferSize=%d must be greater than WindowSize=%d",
			cfg.BufferSize, cfg.WindowSize)
	}
	return nil
}

// Decoder provides a simple buffer for decoding LZ77 sequences. Data is
// the actual buffer. The end of the slice is also the head of the dictionary
// window. R tracks the read position in the buffer and must be less than or
// equal to the length of the Data slice. Off records the total offset and marks
// the end of the Data slice, which is also the end of the dictionary window.
// DecoderConfig provides the configuration parameters WindowSize and
// BufferSize.
type Decoder struct {
	// Data is the actual buffer. The end of the slice is also the head of
	// the dictionary window.
	Data []byte
	// R tracks the position of the reads from the buffer and must be less
	// or equal to the length of the Data slice.
	R int
	// Off records the total offset and marks the end of the Data slice,
	// which is also the end of the dictionary window.
	Off int64

	// DecoderOptions provides the configuration parameters WindowSize and
	// BufferSize.
	DecoderOptions
}

// Init initializes the DecoderBuffer.
func (b *Decoder) Init(opts DecoderOptions) error {
	opts.SetDefaults()
	if err := opts.Verify(); err != nil {
		return err
	}
	*b = Decoder{
		Data:           b.Data[:0],
		DecoderOptions: opts,
	}
	if cap(b.Data) > b.BufferSize {
		b.BufferSize = cap(b.Data)
	}
	return nil
}

// NewDecoder creates and initializes a new Decoder.
func NewDecoder(opts *DecoderOptions) (b *Decoder, err error) {
	b = new(Decoder)
	err = b.Init(*opts)
	return b, err
}

// Reset returns the DecoderBuffer to its initialized state.
func (b *Decoder) Reset() {
	*b = Decoder{
		Data:           b.Data[:0],
		DecoderOptions: b.DecoderOptions,
	}
	if cap(b.Data) > b.BufferSize {
		// The default BufferSize is twice the WindowSize.
		b.BufferSize = cap(b.Data)
	}
}

// ByteAtEnd returns the byte at the end of the buffer.
func (b *Decoder) ByteAtEnd(off int) byte {
	i := len(b.Data) - off
	if !(0 <= i && i < len(b.Data)) {
		return 0
	}
	return b.Data[i]
}

// Read reads decoded data from the buffer.
func (b *Decoder) Read(p []byte) (n int, err error) {
	n = copy(p, b.Data[b.R:])
	b.R += n
	return n, nil
}

// WriteTo writes the decoded data to the writer.
func (b *Decoder) WriteTo(w io.Writer) (n int64, err error) {
	k, err := w.Write(b.Data[b.R:])
	b.R += k
	return int64(k), err
}

// prune evicts data from the buffer and returns the available space.
func (b *Decoder) prune() int {
	// space that can be pruned
	n := min(b.R, max(len(b.Data)-b.WindowSize, 0))
	if n > 0 {
		l := copy(b.Data, b.Data[n:])
		b.Data = b.Data[:l]
		b.R -= n
	}
	return b.BufferSize - len(b.Data)
}

// WriteByte writes a single byte into the buffer.
func (b *Decoder) WriteByte(c byte) error {
	if a := b.BufferSize - len(b.Data); a < 1 {
		if b.prune() < 1 {
			return ErrFullBuffer
		}
	}
	b.Data = append(b.Data, c)
	b.Off++
	return nil
}

// Write inserts the slice into the buffer. The method will write the entire
// slice or return 0 and ErrFullBuffer.
func (b *Decoder) Write(p []byte) (n int, err error) {
	n = len(p)
	if a := b.BufferSize - len(b.Data); n > a {
		if n > b.prune() {
			return 0, ErrFullBuffer
		}
	}
	b.Data = append(b.Data, p...)
	b.Off += int64(n)
	return n, nil
}

// Errors for WriteMatch and WriteBlock.
var (
	errLitLen   = errors.New("lz: LitLen out of range")
	errMatchLen = errors.New("lz: MatchLen out of range")
	errOffset   = errors.New("lz: Offset out of range")
)

// WriteMatch appends the ma tch to the end of the buffer. The match will be
// written completely, or n=0 and ErrFullBuffer will be returned.
func (b *Decoder) WriteMatch(mu, ou uint32) (n int, err error) {
	if ou == 0 && mu > 0 {
		return 0, errOffset
	}
	winLen := min(len(b.Data), b.WindowSize)
	if int64(ou) > int64(winLen) {
		return 0, errOffset
	}
	o := int(ou)
	if int64(mu) > int64(b.WindowSize) {
		return 0, errMatchLen
	}
	m := int(mu)
	if a := b.BufferSize - len(b.Data); m > a {
		if m > b.prune() {
			return 0, ErrFullBuffer
		}
	}
	n = m
	for m > o {
		b.Data = append(b.Data, b.Data[len(b.Data)-o:]...)
		m -= o
		if m <= o {
			break
		}
		o <<= 1
	}
	// m <= o
	i := len(b.Data) - o
	b.Data = append(b.Data, b.Data[i:i+m]...)
	b.Off += int64(n)
	return n, nil
}

// WriteBlock writes sequences from the block into the buffer. Each sequence is
// written atomically, as the block value is not modified. If there is not
// enough space in the buffer, ErrFullBuffer will be returned. All written
// sequences and literals will be removed from the block.
//
// The capacity of the block slices will not be maintained. You have to keep a
// copy of the block to achieve that.
//
// The growth of the array is limited to BufferSize.
//
// The function returns the number of bytes written.
func (b *Decoder) WriteBlock(blk *Block) (n int, err error) {
	var (
		k int
		s Seq
	)
	for k, s = range blk.Sequences {
		if int64(s.LitLen) > int64(len(blk.Literals)) {
			err = errLitLen
			goto end
		}
		l := int(s.LitLen)
		winLen := min(len(b.Data)+l, b.WindowSize)
		if int64(s.Offset) > int64(winLen) {
			err = errOffset
			goto end
		}
		o := int(s.Offset)
		if int64(s.MatchLen) > int64(b.WindowSize) {
			err = errMatchLen
			goto end
		}
		m := int(s.MatchLen)
		if m > 0 && o == 0 {
			err = errOffset
			goto end
		}
		g := l + m
		if g < 0 {
			err = errMatchLen
			goto end
		}
		if a := b.BufferSize - len(b.Data); g > a {
			if g > b.prune() {
				err = ErrFullBuffer
				goto end
			}
		}
		n += g
		b.Data = append(b.Data, blk.Literals[:s.LitLen]...)
		blk.Literals = blk.Literals[s.LitLen:]
		for m > o {
			b.Data = append(b.Data, b.Data[len(b.Data)-o:]...)
			m -= o
			if m <= o {
				break
			}
			o <<= 1
		}
		// m <= o
		i := len(b.Data) - o
		b.Data = append(b.Data, b.Data[i:i+m]...)
	}
	k = len(blk.Sequences)
	if a := b.BufferSize - len(b.Data); len(blk.Literals) > a {
		if len(blk.Literals) > b.prune() {
			err = ErrFullBuffer
			goto end
		}
	}
	b.Data = append(b.Data, blk.Literals...)
	n += len(blk.Literals)
	blk.Literals = blk.Literals[:0]
end:
	blk.Sequences = blk.Sequences[k:]
	b.Off += int64(n)
	return n, err
}
