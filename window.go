package lz

import (
	"errors"
	"io"
)

// Window acts as a buffer and stores the window. Data is written into the
// buffer, the sequencer creates Lempel-Ziv sequences and advances the window
// head. Since all positions behind the window head are in the window we even
// save one check in the sequencer loop.
//
// The window ensures that len(w.data)+7 < cap(w.data), which allows 64-bit
// reads on all byte position of the window.
type Window struct {
	data []byte
	// start stores the absolute position of the window
	start int64
	// w is the position of the window head in data
	w int
	WindowConfig
}

// WindowConfig stores the parameter for the Window.
type WindowConfig struct {
	// WindowSize is the maximum window size in bytes
	WindowSize int
	// ShrinkSize provides the size the window is shrinked to make space for
	// the buffer available
	ShrinkSize int
	// BlockSize provides the block size.
	BlockSize int
}

func (cfg *WindowConfig) ApplyDefaults() {
	if cfg.WindowSize == 0 {
		cfg.WindowSize = 8 * mb
	}
	if cfg.ShrinkSize == 0 {
		const defaultShrinkSize = 32 * kb
		if cfg.WindowSize < 2*defaultShrinkSize {
			cfg.ShrinkSize = cfg.WindowSize / 2
		} else {
			cfg.ShrinkSize = defaultShrinkSize
		}
	}
	if cfg.BlockSize == 0 {
		cfg.BlockSize = 128 * kb
	}
}

func (cfg *WindowConfig) Verify() error {
	if cfg.WindowSize <= 0 {
		return errors.New("lz: window size must be greater than 0")
	}
	if cfg.ShrinkSize < 0 {
		return errors.New("lz: shrink size must be greater or equal 0")
	}
	if cfg.ShrinkSize >= cfg.WindowSize {
		return errors.New(
			"lz: srhink size must be less than the window size")
	}
	if cfg.BlockSize <= 0 {
		return errors.New("lz: block size must be greater than 0")
	}
	return nil
}

// Init initializes the window. The parameter size must be positive.
func (w *Window) Init(cfg WindowConfig) error {
	cfg.ApplyDefaults()
	if err := cfg.Verify(); err != nil {
		return err
	}
	*w = Window{
		data:         w.data[:0],
		WindowConfig: cfg,
	}
	if cap(w.data) < 7 {
		w.data = make([]byte, 0, 1024)
	}
	return nil
}

// Reset cleans the window structure for reuse. Itg will use the data structue
// for the data. Note that the condition cap(data) > len(data) + 7 must be met
// to avoid copying.
func (w *Window) Reset(data []byte) error {
	if data == nil {
		data = w.data[:0]
	}
	if len(data) > w.WindowSize {
		return errors.New("lz: len(data) exceeds window size")
	}
	if len(data)+7 > cap(data) {
		if len(data)+7 <= cap(w.data) {
			w.data = w.data[:len(data)]
		} else {
			w.data = make([]byte, len(w.data), len(w.data)+7)
		}
		copy(w.data, data)
		data = w.data
	}
	*w = Window{
		data:         data,
		WindowConfig: w.WindowConfig,
	}
	if len(w.data)+7 > cap(w.data) {
		panic("unexpected capacity")
	}
	return nil
}

// Available returns the number of bytes are available for writing into the
// buffer.
func (w *Window) Available() int { return w.WindowSize - len(w.data) }

// Buffered returns the number of bytes buffered but are not yet part of the
// window. They have to be sequenced first.
func (w *Window) Buffered() int { return len(w.data) - w.w }

// Len returns the actual length of the current window
func (w *Window) Len() int { return w.w }

// Pos returns the absolute position of the window head
func (w *Window) Pos() int64 { return w.start + int64(w.w) }

// shrink reduces the current window length to n if possible. The method returns
// the actual window length after shrinking.
func (w *Window) shrink() int {
	r := w.w - w.ShrinkSize
	if r <= 0 {
		return w.w
	}

	k := copy(w.data, w.data[r:])
	w.data = w.data[:k]
	w.start += int64(r)
	w.w = w.ShrinkSize
	return w.ShrinkSize
}

// ErrFullBuffer indicates that the buffer is full and no further data can be
// written.
var ErrFullBuffer = errors.New("lz: full buffer")

// Write puts data into the window. It will return ErrFullBuffer
func (w *Window) Write(p []byte) (n int, err error) {
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
		if k > w.WindowSize {
			k = w.WindowSize + 7
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
func (w *Window) ReadFrom(r io.Reader) (n int64, err error) {
	if len(w.data) == w.WindowSize {
		return 0, ErrFullBuffer
	}
	for {
		var p []byte
		if w.WindowSize <= cap(w.data)-7 {
			p = w.data[len(w.data):w.WindowSize]
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
		if len(w.data) == w.WindowSize {
			return n, ErrFullBuffer
		}
		k := 2 * cap(w.data)
		if k < 1024 {
			k = 1024
		}
		if k > w.WindowSize {
			k = w.WindowSize + 7
		}
		t := make([]byte, len(w.data), k)
		copy(t, w.data)
		w.data = t
	}
}

var errOutsideWindow = errors.New("lz: pos outside of window buffer")

// ReadByteAt returns the byte at the absolute position pos unless pos is outside of
// the data stored in window.
func (w *Window) ReadByteAt(pos int64) (c byte, err error) {
	pos -= w.start
	if !(0 <= pos && pos < int64(len(w.data))) {
		return 0, errOutsideWindow
	}
	return w.data[pos], nil
}

// ReadAt allows to read data from the window directly.
func (w *Window) ReadAt(p []byte, pos int64) (n int, err error) {
	pos -= w.start
	if !(0 <= pos && pos < int64(len(w.data))) {
		return 0, errOutsideWindow
	}
	n = copy(p, w.data[pos:])
	return n, nil
}

func (w *Window) additionalMemSize() uintptr {
	return uintptr(cap(w.data))
}
