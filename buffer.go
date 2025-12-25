package lz

import (
	"errors"
	"fmt"
	"io"
)

// ShiftFunc defines the type of the shift function, called when the buffer is
// pruned to provide more available space.
type ShiftFunc func(delta int)

// Buffer is the buffer used for LZ parsing.
//
// The Off field describes the offset of Data[0] in the original stream. The W
// points to the end of sliding window used for copying matches.
//
// Data is not fully allocated at the beginning. It grows with the usage. There
// must be always 7 extra bytes allocated at the end of Data to allow easy reads
// of data from the buffer.
type Buffer struct {
	Data []byte
	// Window end index
	W int
	// maximum buffer size
	Size int
	// RetentionSize is the number of bytes to keep when pruning the buffer.
	RetentionSize int
	// offset of Data
	Off int64

	// Shift will be called when the buffer is pruned to inform other
	// components about the number of bytes removed from the start of
	// the buffer.
	Shift ShiftFunc
}

// Init initializes the buffer. The old data slice is reused and the capacity
// might be larger than the new buffer size.
func (b *Buffer) Init(size, retentionSize int, shift ShiftFunc) error {
	if !(0 < size) {
		return fmt.Errorf("lz: invalid buffer size: %d", size)
	}
	if !(0 <= retentionSize && retentionSize <= size) {
		return fmt.Errorf("lz: invalid retention size: %d; bufferSize %d",
			retentionSize, size)
	}
	*b = Buffer{
		Data:          b.Data[:0],
		Size:          size,
		RetentionSize: retentionSize,
		Shift:         shift,
	}
	return nil
}

// Reset resets the buffer with the provided data slice. If the data slice is
// larger than the buffer size, the buffer size will be updated. Note that the
// data slice should have 7 extra bytes, len(data)+7 <= cap(data). Otherwise the
// old slice will be used or a new one need to be allocated.
func (b *Buffer) Reset(data []byte) error {
	if len(data) > b.Size {
		b.Size = len(data)
	}
	switch {
	case len(data) <= cap(data)-7:
		b.Data = data
	case len(data) <= cap(b.Data)-7:
		b.Data = b.Data[:len(data)]
		copy(b.Data, data)
	default:
		b.Data = make([]byte, len(data), len(data)+7)
		copy(b.Data, data)
	}
	b.W = 0
	b.Off = 0
	return nil
}

// Parsable returns the number of bytes that can be parsed from the buffer.
func (b *Buffer) Parsable() int {
	return len(b.Data) - b.W
}

// makeAvailable returns the slice of available bytes and should be larger or
// equal the parameter n. The returned slice might be smaller than n if the
// buffer reaches the buffer size limit.
func (b *Buffer) makeAvailable(n int) []byte {
	n = max(n, 0)
	k := len(b.Data)
	l := min(k+n, b.Size)
	if l > cap(b.Data)-7 {
		c := min(max(2*cap(b.Data), 1024, l), b.Size)
		p := make([]byte, k, c+7)
		copy(p, b.Data)
		b.Data = p
	}
	return b.Data[k:min(cap(b.Data)-7, b.Size)]
}

// ErrFullBuffer is returned when the buffer is full and no more data can be
// written to it.
var ErrFullBuffer = errors.New("lz: full buffer")

// Write writes data to the buffer. If not all data can be written, ErrFullBuffer
// is returned.
func (b *Buffer) Write(p []byte) (n int, err error) {
	n = len(p)
	if n == 0 {
		return 0, nil
	}
	if len(b.Data)+n > b.Size {
		b.prune()
	}
	n = copy(b.makeAvailable(n), p)
	b.Data = b.Data[:len(b.Data)+n]
	if n < len(p) {
		err = ErrFullBuffer
	}
	return n, err
}

// ReadFrom reads data from r until EOF or error. It returns the number of bytes
// read and any error encountered.
func (b *Buffer) ReadFrom(r io.Reader) (n int64, err error) {
	const chunkSize = 32 << 10
	for {
		if len(b.Data)+chunkSize >= b.Size {
			b.prune()
		}
		p := b.makeAvailable(chunkSize)
		if len(p) == 0 {
			return n, ErrFullBuffer
		}
		k, err := r.Read(p)
		n += int64(k)
		b.Data = b.Data[:len(b.Data)+k]
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return n, err
		}
	}
}

// ErrOutOfBuffer is returned when the offset is outside of the buffer.
var ErrOutOfBuffer = errors.New("lz: offset outside of buffer")

// ReadAt reads len(p) bytes from the buffer starting at byte offset off. It
// returns the number of bytes read and any error encountered. If off is outside
// of the buffer, ErrOutOfBuffer is returned.
func (b *Buffer) ReadAt(p []byte, off int64) (n int, err error) {
	i := off - b.Off
	if !(0 <= i && i < int64(len(b.Data))) {
		return 0, ErrOutOfBuffer
	}
	n = copy(p, b.Data[i:])
	if n < len(p) {
		err = io.EOF
	}
	return n, err
}

// ByteAt returns the byte at offset off. If off is outside of the buffer,
// ErrOutOfBuffer is returned.
func (b *Buffer) ByteAt(off int64) (c byte, err error) {
	i := off - b.Off
	if !(0 <= i && i < int64(len(b.Data))) {
		return 0, ErrOutOfBuffer
	}
	return b.Data[i], nil
}

// prune cuts bytes from the start of the buffer to make more space available.
// The number of bytes to keep can be provided. If the parameter is zero, 25% of
// the buffer size is kept. If the parameter is negative it is handled like the
// zero value.
func (b *Buffer) prune() int {
	n := max(b.W-b.RetentionSize, 0)
	if n == 0 {
		return 0
	}
	k := copy(b.Data, b.Data[n:])
	b.Data = b.Data[:k]
	b.Off += int64(n)
	b.W -= n
	if b.Shift != nil {
		b.Shift(n)
	}
	return n
}
