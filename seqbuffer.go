package lz

import (
	"errors"
	"fmt"
	"io"
)

// SeqBuffer provides a base for Sequencer implementation. Since the package
// allows implementations outside of the package. All members are public.
type SeqBuffer struct {
	// actual buffer data
	Data []byte

	// w position of the head of the window in data.
	W int

	// off start of the data slice, counts all data written and discarded
	// from the buffer.
	Off int64

	BufConfig
}

// Init initializes the buffer. The function
// sets the defaults for the buffer configuration if required and verifies it.
// Errors will be reported.
func (b *SeqBuffer) Init(cfg BufConfig) error {
	cfg.SetDefaults()
	var err error
	if err = cfg.Verify(); err != nil {
		return err
	}
	*b = SeqBuffer{
		Data:      b.Data[:0],
		BufConfig: cfg,
	}
	return err
}

// Reset initializes the buffer with new data. The data slice requires a margin
// of 7 bytes for the hash sequencers to be used directly. If there is no margin
// the data will be copied into a slice with enough capacity.
func (b *SeqBuffer) Reset(data []byte) error {
	if len(data) > b.BufferSize {
		return fmt.Errorf("lz: len(data)=%d larger than BufferSize=%d",
			len(data), b.BufferSize)
	}

	b.W = 0
	b.Off = 0

	if len(data) == 0 {
		b.Data = b.Data[:0]
		return nil
	}

	// Ensure the margin required for the hash sequencers.
	margin := len(data) + 7
	if margin > cap(data) {
		if margin > cap(b.Data) {
			b.Data = make([]byte, len(data), margin)
		} else {
			b.Data = b.Data[:len(data)]
		}
		copy(b.Data, data)
	} else {
		b.Data = data
	}

	return nil
}

// Shrink will move the window head to the shrink size if it is larger. The
// amount of data discarded from the buffer, named delta, will be returned.
func (b *SeqBuffer) Shrink() int {
	delta := b.W - b.ShrinkSize
	if delta <= 0 {
		return 0
	}
	n := copy(b.Data, b.Data[delta:])
	b.Data = b.Data[:n]
	b.W = b.ShrinkSize
	b.Off += int64(delta)
	return delta
}

// grow will allocate more buffer data that will have enough space for t bytes
// or BufferSize bytes plus 7 bytes margin to support the hash sequencers.
// Usually the size allocate will roughly more than twice the requested size to
// avoid repeated allocations.
func (b *SeqBuffer) grow(t int) {
	if t+7 <= cap(b.Data) {
		return
	}

	c := 2 * t
	if 0 <= c && c < 1024 {
		c = 1024
	} else {
		c = ((c + 1<<10 - 1) >> 10) << 10
	}
	if c >= b.BufferSize+7 || c < 0 {
		c = b.BufferSize + 7
	}
	// Allocate the buffer.
	p := b.Data
	b.Data = make([]byte, len(b.Data), c)
	copy(b.Data, p)
}

// Write writes data into the buffer. If not the complete p slice can be copied
// into the buffer, Write will return [ErrFullBuffer].
func (b *SeqBuffer) Write(p []byte) (n int, err error) {
	available := b.BufferSize - len(b.Data)
	if available < len(p) {
		p = p[:available]
		err = ErrFullBuffer
	}
	n = len(p)

	t := len(b.Data) + n
	if t+7 > cap(b.Data) {
		b.grow(t)
	}
	b.Data = append(b.Data, p...)
	return n, err
}

// ReadFrom reads the data from reader into the buffer. If there is an error it
// will be reported. If the buffer is full, [ErrFullBuffer] will be reported.
func (b *SeqBuffer) ReadFrom(r io.Reader) (n int64, err error) {
	const chunkSize = 32 << 10
	n = int64(len(b.Data))
	for {
		if len(b.Data) >= b.BufferSize {
			err = ErrFullBuffer
			break
		}
		t := min(len(b.Data)+chunkSize, b.BufferSize)
		if t+7 > cap(b.Data) {
			b.grow(t)
		}
		p := b.Data[len(b.Data) : cap(b.Data)-7]
		var k int
		k, err = r.Read(p)
		b.Data = b.Data[:len(b.Data)+k]
		if err != nil {
			break
		}
	}
	return int64(len(b.Data)) - n, err
}

// Errors returned by [SeqBuffer.ReadAt]
var (
	ErrOutOfBuffer = errors.New("lz: offset out of buffer")
	ErrEndOfBuffer = errors.New("lz: end of buffer")
)

// ReadAt reads data from the buffer at position off. If off is is outside the
// buffer ErrOutOfBuffer will be reported. If there is not enough data to fill p
// ErrEndOfBuffer will be reported. See [SeqBuffer.PeekAt] for avoiding the
// copy.
func (b *SeqBuffer) ReadAt(p []byte, off int64) (n int, err error) {
	q, err := b.PeekAt(len(p), off)
	n = copy(p, q)
	return n, err
}

// PeekAt returns part of the internal data slice starting at total offset off.
// The total offset takes all data written to the buffer into account. If the
// off parameter is outside the current buffer ErrOutOfBuffer will be returned.
// If less than n bytes of data can be provided ErrEndOfBuffer will be returned.
func (b *SeqBuffer) PeekAt(n int, off int64) (p []byte, err error) {
	i := off - b.Off
	if !(0 <= i && i < int64(len(b.Data))) {
		return nil, ErrOutOfBuffer
	}
	p = b.Data[i:]
	if len(p) < n {
		err = ErrEndOfBuffer
	}
	return p, err
}

// ByteAt returns the byte at total offset off, if it can be provided. If off
// points to the end of the buffer, [ErrEndOfBuffer] will be returned otherwise
// [ErrOutOfBuffer].
func (b *SeqBuffer) ByteAt(off int64) (c byte, err error) {
	i := off - b.Off
	if !(0 <= i && i <= int64(len(b.Data))) {
		if i == int64(len(b.Data)) {
			return 0, ErrEndOfBuffer
		}
		return 0, ErrOutOfBuffer
	}
	return b.Data[i], nil
}
