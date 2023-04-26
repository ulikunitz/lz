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
	data []byte

	// w position of the head of the window in data.
	w int

	// off start of the data slice, counts all data written and discarded
	// from the buffer.
	off int64

	cfg BufConfig
}

// Init initializes the buffer and sets its data field to data. The function
// sets the defaults for the buffer configuration if required and verifies it.
// Errors will be reported. The data field must be less than the buffer size
// otherwise an error will be reported.
func (s *SeqBuffer) Init(cfg BufConfig, data []byte) error {
	cfg.SetDefaults()
	var err error
	if err = cfg.Verify(); err != nil {
		return err
	}
	s.cfg = cfg
	err = s.Reset(data)
	return err
}

// BufferConfig returns the configuration of the buffer.
func (b *SeqBuffer) BufferConfig() BufConfig { return b.cfg }

// Bytes returns the content of the buffer.
func (b *SeqBuffer) Bytes() []byte { return b.data }

// W returns the position of the window head in the slice returned by
// [SeqBuffer.Bytes].
func (b *SeqBuffer) W() int { return b.w }

// SetW sets the new w. It must be larger or equal the current w and less than
// or equal the length of the buffer data. The method panics if that is not the
// case.
func (b *SeqBuffer) SetW(w int) {
	if !(b.w <= w && w <= len(b.data)) {
		panic(fmt.Errorf("lz: w=%d outside of range [%d..%d]",
			w, b.w, len(b.data)))
	}
	b.w = w
}

// Resets the buffer to the the new data. Note that the buffer will try to use p
// as internal data slice if possible to avoid copying.
func (b *SeqBuffer) Reset(data []byte) error {
	if len(data) > b.cfg.BufferSize {
		return fmt.Errorf("lz: len(data)=%d larger than BufferSize=%d",
			len(data), b.cfg.BufferSize)
	}

	b.w = 0
	b.off = 0

	// Ensure the margin required for the hash sequencers.
	margin := len(data) + 7
	if margin > cap(data) {
		if margin > cap(b.data) {
			b.data = make([]byte, len(data), margin)
		} else {
			b.data = b.data[:len(data)]
		}
		copy(b.data, data)
	} else {
		b.data = data
	}

	return nil
}

// Shrink will move the window head to the shrink size if it is larger. The
// amount of data discarded from the buffer, named delta, will be returned.
func (b *SeqBuffer) Shrink() int {
	delta := b.w - b.cfg.ShrinkSize
	if delta <= 0 {
		return 0
	}
	n := copy(b.data, b.data[delta:])
	b.data = b.data[:n]
	b.w = b.cfg.ShrinkSize
	b.off += int64(delta)
	return delta
}

// grow will allocate more buffer data that will have enough space for t bytes
// or BufferSize bytes plus 7 bytes margin to support the hash sequencers.
// Usually the size allocate will roughly more than twice the requested size to
// avoid repeated allocations.
func (b *SeqBuffer) grow(t int) {
	if t+7 <= cap(b.data) {
		return
	}

	c := 2 * t
	if 0 <= c && c < 1024 {
		c = 1024
	} else {
		c = ((c + 1<<10 - 1) >> 10) << 10
	}
	if c >= b.cfg.BufferSize || c < 0 {
		c = b.cfg.BufferSize + 7
	}
	// Allocate the buffer.
	p := b.data
	b.data = make([]byte, len(b.data), c)
	copy(b.data, p)
}

// Write writes data into the buffer. If not the complete p slice can be copied
// into the buffer, Write will return [ErrFullBuffer].
func (b *SeqBuffer) Write(p []byte) (n int, err error) {
	available := b.cfg.BufferSize - len(b.data)
	if available < len(p) {
		p = p[:available]
		err = ErrFullBuffer
	}
	n = len(p)

	t := len(b.data) + n
	if t+7 > cap(b.data) {
		b.grow(t)
	}
	b.data = append(b.data, p...)
	return n, err
}

// ReadFrom reads the data from reader into the buffer. If there is an error it
// will be reported. If the buffer is full, [ErrFullBuffer] will be reported.
func (b *SeqBuffer) ReadFrom(r io.Reader) (n int64, err error) {
	const chunkSize = 32 << 10
	n = int64(len(b.data))
	for {
		t := min(len(b.data)+chunkSize, b.cfg.BufferSize)
		if t+7 > cap(b.data) {
			b.grow(t)
		}
		p := b.data[len(b.data) : cap(b.data)-7]
		var k int
		k, err = r.Read(p)
		b.data = b.data[:len(b.data)+k]
		if err != nil {
			break
		}
		if len(b.data) >= b.cfg.BufferSize {
			err = ErrFullBuffer
			break
		}
	}
	return int64(len(b.data)) - n, err
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
	i := off - b.off
	if !(0 <= i && i < int64(len(b.data))) {
		return nil, ErrOutOfBuffer
	}
	p = b.data[i:]
	if len(p) < n {
		err = ErrEndOfBuffer
	}
	return p, err
}

// ByteAt returns the byte at total offset off, if it can be provided. If off
// points to the end of the buffer, [ErrEndOfBuffer] will be returned otherwise
// [ErrOutOfBuffer].
func (b *SeqBuffer) ByteAt(off int64) (c byte, err error) {
	i := off - b.off
	if !(0 <= i && i <= int64(len(b.data))) {
		if i == int64(len(b.data)) {
			return 0, ErrEndOfBuffer
		}
		return 0, ErrOutOfBuffer
	}
	return b.data[i], nil
}
