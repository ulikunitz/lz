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

func (opts DecoderOptions) NewDecoder() (*Decoder, error) {
	d := &Decoder{}
	if err := d.Init(opts); err != nil {
		return nil, err
	}
	return d, nil
}

// setDefaults assigns default values to zero fields in DecoderConfig.
func (opts *DecoderOptions) setDefaults() {
	if opts.WindowSize == 0 {
		opts.WindowSize = 8 << 20
	}
	if opts.BufferSize == 0 {
		opts.BufferSize = 2 * opts.WindowSize
	}
}

// verify checks the parameters of the DecoderConfig value and returns an error
// for the first issue found.
func (opts *DecoderOptions) verify() error {
	if !(0 <= opts.WindowSize && opts.WindowSize <= math.MaxUint32) {
		return fmt.Errorf(
			"lz.DecConfig: WindowSize=%d out of range [%d..%d]",
			opts.WindowSize, 0, math.MaxUint32)
	}
	if !(opts.WindowSize < opts.BufferSize) {
		return fmt.Errorf(
			"lz.DecConfig: BufferSize=%d must be greater than WindowSize=%d",
			opts.BufferSize, opts.WindowSize)
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
func (d *Decoder) Init(opts DecoderOptions) error {
	opts.setDefaults()
	if err := opts.verify(); err != nil {
		return err
	}
	*d = Decoder{
		Data:           d.Data[:0],
		DecoderOptions: opts,
	}
	if cap(d.Data) > d.BufferSize {
		d.BufferSize = cap(d.Data)
	}
	return nil
}

// Reset returns the DecoderBuffer to its initialized state.
func (d *Decoder) Reset() {
	*d = Decoder{
		Data:           d.Data[:0],
		DecoderOptions: d.DecoderOptions,
	}
	if cap(d.Data) > d.BufferSize {
		// The default BufferSize is twice the WindowSize.
		d.BufferSize = cap(d.Data)
	}
}

// ByteAtEnd returns the byte at the end of the buffer.
func (d *Decoder) ByteAtEnd(off int) byte {
	i := len(d.Data) - off
	if !(0 <= i && i < len(d.Data)) {
		return 0
	}
	return d.Data[i]
}

// Read reads decoded data from the buffer.
func (d *Decoder) Read(p []byte) (n int, err error) {
	n = copy(p, d.Data[d.R:])
	d.R += n
	return n, nil
}

// WriteTo writes the decoded data to the writer.
func (d *Decoder) WriteTo(w io.Writer) (n int64, err error) {
	k, err := w.Write(d.Data[d.R:])
	d.R += k
	return int64(k), err
}

// prune evicts data from the buffer and returns the available space.
func (d *Decoder) prune() int {
	// space that can be pruned
	n := min(d.R, max(len(d.Data)-d.WindowSize, 0))
	if n > 0 {
		l := copy(d.Data, d.Data[n:])
		d.Data = d.Data[:l]
		d.R -= n
	}
	return d.BufferSize - len(d.Data)
}

// WriteByte writes a single byte into the buffer.
func (d *Decoder) WriteByte(c byte) error {
	if a := d.BufferSize - len(d.Data); a < 1 {
		if d.prune() < 1 {
			return ErrFullBuffer
		}
	}
	d.Data = append(d.Data, c)
	d.Off++
	return nil
}

// Write inserts the slice into the buffer. The method will write the entire
// slice or return 0 and ErrFullBuffer.
func (d *Decoder) Write(p []byte) (n int, err error) {
	n = len(p)
	if a := d.BufferSize - len(d.Data); n > a {
		if n > d.prune() {
			return 0, ErrFullBuffer
		}
	}
	d.Data = append(d.Data, p...)
	d.Off += int64(n)
	return n, nil
}

// Errors for WriteMatch and WriteBlock.
var (
	errLitLen   = errors.New("lz: LitLen out of range")
	errMatchLen = errors.New("lz: MatchLen out of range")
	errOffset   = errors.New("lz: Offset out of range")
)

// WriteMatch appends the match to the end of the buffer. The match will be
// written completely, or n=0 and ErrFullBuffer will be returned.
func (d *Decoder) WriteMatch(mu, ou uint32) (n int, err error) {
	if ou == 0 && mu > 0 {
		return 0, errOffset
	}
	winLen := min(len(d.Data), d.WindowSize)
	if int64(ou) > int64(winLen) {
		return 0, errOffset
	}
	o := int(ou)
	if int64(mu) > int64(d.WindowSize) {
		return 0, errMatchLen
	}
	m := int(mu)
	if a := d.BufferSize - len(d.Data); m > a {
		if m > d.prune() {
			return 0, ErrFullBuffer
		}
	}
	n = m
	for m > o {
		d.Data = append(d.Data, d.Data[len(d.Data)-o:]...)
		m -= o
		if m <= o {
			break
		}
		o <<= 1
	}
	// m <= o
	i := len(d.Data) - o
	d.Data = append(d.Data, d.Data[i:i+m]...)
	d.Off += int64(n)
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
func (d *Decoder) WriteBlock(blk *Block) (n int, err error) {
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
		winLen := min(len(d.Data)+l, d.WindowSize)
		if int64(s.Offset) > int64(winLen) {
			err = errOffset
			goto end
		}
		o := int(s.Offset)
		if int64(s.MatchLen) > int64(d.WindowSize) {
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
		if a := d.BufferSize - len(d.Data); g > a {
			if g > d.prune() {
				err = ErrFullBuffer
				goto end
			}
		}
		n += g
		d.Data = append(d.Data, blk.Literals[:s.LitLen]...)
		blk.Literals = blk.Literals[s.LitLen:]
		for m > o {
			d.Data = append(d.Data, d.Data[len(d.Data)-o:]...)
			m -= o
			if m <= o {
				break
			}
			o <<= 1
		}
		// m <= o
		i := len(d.Data) - o
		d.Data = append(d.Data, d.Data[i:i+m]...)
	}
	k = len(blk.Sequences)
	if a := d.BufferSize - len(d.Data); len(blk.Literals) > a {
		if len(blk.Literals) > d.prune() {
			err = ErrFullBuffer
			goto end
		}
	}
	d.Data = append(d.Data, blk.Literals...)
	n += len(blk.Literals)
	blk.Literals = blk.Literals[:0]
end:
	blk.Sequences = blk.Sequences[k:]
	d.Off += int64(n)
	return n, err
}
