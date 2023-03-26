package lz

import (
	"errors"
	"fmt"
	"io"
)

// Constants for kilobytes and megabytes.
const (
	kb = 1 << 10
	mb = 1 << 20
)

// SeqBuffer acts as a buffer for the sequencers. The buffer contains the window
// from which matches can't be copied in a sequence. Data is written into the
// buffer, the sequencer creates Lempel-Ziv sequences and advances the window
// head. Since all positions behind the window head are in the window we even
// save one check in the sequencer loop.
//
// The Sequencer  ensures that len(w.data)+7 < cap(w.data), which allows 64-bit
// reads on all byte position of the window.
type SeqBuffer struct {
	data []byte
	// start stores the absolute position of the start of the data slice.
	start int64
	// w is the position of the window head in data slice.
	w int
	// SBConfig stores the configuration parameters
	SBConfig
}

// SBConfig stores the parameter for the Window.
type SBConfig struct {
	// WindowSize is the maximum window size in bytes
	WindowSize int
	// ShrinkSize provides the size the buffer is shrunk to if the buffer
	// has been completely filled and encoded. It must be smaller than the
	// BufferSize, and should be significantly so.
	ShrinkSize int
	// BufferSize defines the maximum size of the buffer. The BufferSize
	// must be greater or equal the window size.
	BufferSize int

	// BlockSize provides the block size.
	BlockSize int
}

// BufferConfig returns the a pointer to the sequencer buffer configuration,
// SBConfig.
func (cfg *SBConfig) BufferConfig() *SBConfig { return cfg }

const defaultShrinkSize = 32 * kb

// ApplyDefaults sets the defaults for the sequencer buffer configuration.
func (cfg *SBConfig) ApplyDefaults() {
	if cfg.WindowSize == 0 {
		cfg.WindowSize = 8 * mb
	}
	if cfg.ShrinkSize == 0 {
		if 2*defaultShrinkSize > cfg.WindowSize {
			cfg.ShrinkSize = cfg.WindowSize / 2
		} else {
			cfg.ShrinkSize = defaultShrinkSize
		}
	}
	if cfg.BufferSize == 0 {
		cfg.BufferSize = cfg.WindowSize
	}
	if cfg.BufferSize < cfg.WindowSize || cfg.BufferSize <= cfg.ShrinkSize {
		cfg.BufferSize = 2 * cfg.ShrinkSize
		if cfg.BufferSize < cfg.WindowSize {
			cfg.BufferSize = cfg.WindowSize
		}
	}
	if cfg.BlockSize == 0 {
		cfg.BlockSize = 128 * kb
	}
}

// SetWindowSize sets the window size. BufferSize and ShrinkSize will be
// adapted.
func (cfg *SBConfig) SetWindowSize(s int) error {
	if !(0 < s) {
		return fmt.Errorf("lz: window size %d is larger zero", s)
	}
	cfg.WindowSize = s
	cfg.BufferSize = cfg.WindowSize
	if 2*defaultShrinkSize > cfg.WindowSize {
		cfg.ShrinkSize = cfg.WindowSize / 2
	} else {
		cfg.ShrinkSize = defaultShrinkSize
	}
	return nil
}

// Verify checks the sequencer buffer configuration for issues and returns the
// first issue found as error.
func (cfg *SBConfig) Verify() error {
	if cfg.WindowSize <= 0 {
		return errors.New("lz: window size must be greater than 0")
	}
	if cfg.ShrinkSize < 0 {
		return errors.New("lz: shrink size must be greater or equal 0")
	}
	if cfg.BufferSize < cfg.WindowSize {
		return errors.New(
			"lz: buffer size must greater or equal window size")
	}
	if cfg.ShrinkSize >= cfg.BufferSize {
		return errors.New(
			"lz: shrink size must be less than buffer size")
	}

	if cfg.BlockSize <= 0 {
		return errors.New("lz: block size must be greater than 0")
	}
	return nil
}

// Init initializes the window. The parameter size must be positive.
func (w *SeqBuffer) Init(cfg SBConfig) error {
	cfg.ApplyDefaults()
	if err := cfg.Verify(); err != nil {
		return err
	}
	*w = SeqBuffer{
		data:     w.data[:0],
		SBConfig: cfg,
	}
	if cap(w.data) < 7 {
		w.data = make([]byte, 0, 1024)
	}
	return nil
}

// Reset cleans the window structure for reuse. It will use the data structure
// for the data. Note that the condition cap(data) > len(data) + 7 must be met
// to avoid copying. The data length must not exceed the buffer size.
func (w *SeqBuffer) Reset(data []byte) error {
	if data == nil {
		data = w.data[:0]
	}
	if len(data) > w.BufferSize {
		return fmt.Errorf(
			"lz: length of the reset data block (%d)"+
				" must not be larger than the buffer size (%d)",
			len(data), w.BufferSize)
	}
	if len(data)+7 > cap(data) {
		if len(data)+7 <= cap(w.data) {
			w.data = w.data[:len(data)]
		} else {
			w.data = make([]byte, len(data), len(data)+7)
		}
		copy(w.data, data)
		data = w.data
	}
	*w = SeqBuffer{
		data:     data,
		SBConfig: w.SBConfig,
	}
	if len(w.data)+7 > cap(w.data) {
		panic("unexpected capacity")
	}
	return nil
}

// Available returns the number of bytes are available for writing into the
// buffer.
func (w *SeqBuffer) Available() int {
	n := w.BufferSize - len(w.data)
	if n < 0 {
		return 0
	}
	return n
}

// Buffered returns the number of bytes buffered but are not yet part of the
// window. They have to be sequenced first.
func (w *SeqBuffer) Buffered() int { return len(w.data) - w.w }

// Len returns the actual length of the current window
func (w *SeqBuffer) Len() int {
	if w.w > w.WindowSize {
		return w.WindowSize
	}
	return w.w
}

// Pos returns the absolute position of the window head
func (w *SeqBuffer) Pos() int64 { return w.start + int64(w.w) }

// shrink reduces the current window length. The method returns the non-negative
// delta that the window has been shifted. 
func (w *SeqBuffer) shrink() int {
	r := w.w - w.ShrinkSize
	if r <= 0 {
		return 0
	}

	k := copy(w.data, w.data[r:])
	w.data = w.data[:k]
	w.start += int64(r)
	w.w = w.ShrinkSize
	return r
}

// ErrFullBuffer indicates that the buffer is full and no further data can be
// written.
var ErrFullBuffer = errors.New("lz: full buffer")

// Write puts data into the window. It will return ErrFullBuffer
func (w *SeqBuffer) Write(p []byte) (n int, err error) {
	n = w.Available()
	if n < len(p) {
		p = p[:n]
		err = ErrFullBuffer
	}
	n = len(w.data) + len(p)
	if n+7 > cap(w.data) {
		k := 2 * cap(w.data)
		if k < 1024 {
			k = 1024
		}
		if k > w.BufferSize {
			k = w.BufferSize + 7
		}
		if n+7 > k {
			k = n + 7
		}

		t := make([]byte, n, k)
		copy(t, w.data)
		copy(t[len(w.data):], p)
		w.data = t
	} else {
		w.data = append(w.data, p...)
	}
	return len(p), err
}

// ReadFrom transfers data from the reader into the buffer.
func (w *SeqBuffer) ReadFrom(r io.Reader) (n int64, err error) {
	if len(w.data) >= w.BufferSize {
		return 0, ErrFullBuffer
	}
	for {
		var p []byte
		if w.BufferSize <= cap(w.data)-7 {
			p = w.data[len(w.data):w.BufferSize]
		} else {
			p = w.data[len(w.data) : cap(w.data)-7]
		}
		for len(p) > 0 {
			k, err := r.Read(p)
			n += int64(k)
			w.data = w.data[:len(w.data)+k]
			if err != nil {
				if err == io.EOF {
					err = nil
				}
				return n, err
			}
			p = p[k:]
		}
		if len(w.data) == w.BufferSize {
			return n, ErrFullBuffer
		}
		k := 2 * cap(w.data)
		if k < 1024 {
			k = 1024
		}
		if k > w.BufferSize {
			k = w.BufferSize + 7
		}
		t := make([]byte, len(w.data), k)
		copy(t, w.data)
		w.data = t
	}
}

// errOutsideBuffer indicates that a position value points actually outside the
// buffer.
var errOutsideBuffer = errors.New("lz: pos outside of sequencer buffer")

// ReadByteAt returns the byte at the absolute position pos unless pos is outside of
// the data stored in window.
func (w *SeqBuffer) ReadByteAt(pos int64) (c byte, err error) {
	pos -= w.start
	if !(0 <= pos && pos < int64(len(w.data))) {
		return 0, errOutsideBuffer
	}
	return w.data[pos], nil
}

// ReadAt allows to read data from the window directly.
func (w *SeqBuffer) ReadAt(p []byte, pos int64) (n int, err error) {
	pos -= w.start
	if !(0 <= pos && pos < int64(len(w.data))) {
		return 0, errOutsideBuffer
	}
	n = copy(p, w.data[pos:])
	return n, nil
}

// additionalMemSize returns the memory that is additionally used by this
// structure.
func (w *SeqBuffer) additionalMemSize() uintptr {
	return uintptr(cap(w.data))
}

// Buffer returns a pointer to itself. It provides the function to the sequencer
// structure who embed SeqBuffer.
func (w *SeqBuffer) Buffer() *SeqBuffer { return w }
