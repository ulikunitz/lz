package lz

import (
	"errors"
	"fmt"
	"io"
)

// SeqWriter acts as a buffer for the sequencers. The buffer contains the window
// from which matches can't be copied in a sequence. Data is written into the
// buffer, the sequencer creates Lempel-Ziv sequences and advances the window
// head. Since all positions behind the window head are in the window we even
// save one check in the sequencer loop.
//
// The Sequencer  ensures that len(w.data)+7 < cap(w.data), which allows 64-bit
// reads on all byte position of the window.
type SeqWriter struct {
	data []byte
	// start stores the absolute position of the start of the data slice.
	start int64
	// w is the position of the window head in data slice.
	w int

	seq Sequencer

	BufConfig
}

// ensureMargin ensures that there is a margin of 7 bytes following the data
// length allowing to extend the byte slice for access by _getLE64.
func ensureMargin(p []byte) []byte {
	k := len(p) + 7
	if cap(p) >= k {
		return p
	}
	if k < 1024 {
		k = 1024
	}
	q := make([]byte, len(p), k)
	copy(q, p)
	return q
}

// Init initializes the window. The parameter size must be positive.
func (w *SeqWriter) Init(seq Sequencer, data []byte) error {
	bc := BC(seq)
	if len(data) > bc.BufferSize {
		return fmt.Errorf(
			"lz: length of the reset data block (%d)"+
				" must not be larger than the buffer size (%d)",
			len(data), bc.BufferSize)
	}
	*w = SeqWriter{
		data:      data,
		seq:       seq,
		BufConfig: bc,
	}
	w.data = ensureMargin(w.data)
	seq.Update(data, -1)
	return nil
}

// Reset cleans the window structure for reuse. It will use the data structure
// for the data. Note that the condition cap(data) > len(data) + 7 must be met
// to avoid copying. The data length must not exceed the buffer size.
func (w *SeqWriter) Reset(data []byte) error {
	if data == nil {
		data = w.data[:0]
	}
	if len(data) > w.BufferSize {
		return fmt.Errorf(
			"lz: length of the reset data block (%d)"+
				" must not be larger than the buffer size (%d)",
			len(data), w.BufferSize)
	}
	k := len(data) + 7
	if k > cap(data) {
		if cap(w.data) >= k {
			w.data = w.data[:len(data)]
			copy(w.data, data)
			data = w.data
		} else {
			data = ensureMargin(data)
		}
	}
	*w = SeqWriter{
		data:      data,
		seq:       w.seq,
		BufConfig: w.BufConfig,
	}
	if len(w.data)+7 > cap(w.data) {
		panic("unexpected capacity")
	}
	w.seq.Update(w.data, -1)
	return nil
}

// Available returns the number of bytes are available for writing into the
// buffer.
func (w *SeqWriter) Available() int {
	n := w.BufferSize - len(w.data)
	if n < 0 {
		return 0
	}
	return n
}

// Buffered returns the number of bytes buffered but are not yet part of the
// window. They have to be sequenced first.
func (w *SeqWriter) Buffered() int { return len(w.data) - w.w }

// Len returns the actual length of the current window
func (w *SeqWriter) Len() int {
	if w.w > w.WindowSize {
		return w.WindowSize
	}
	return w.w
}

// Pos returns the absolute position of the window head
func (w *SeqWriter) Pos() int64 { return w.start + int64(w.w) }

// shrink reduces the current window length. The method returns the non-negative
// delta that the window has been shifted.
func (w *SeqWriter) Shrink() int {
	delta := w.w - w.ShrinkSize
	if delta <= 0 {
		return 0
	}

	k := copy(w.data, w.data[delta:])
	w.data = w.data[:k]
	w.start += int64(delta)
	w.w = w.ShrinkSize
	w.seq.Update(w.data, delta)
	return delta
}

// Write puts data into the window. It will return ErrFullBuffer
func (w *SeqWriter) Write(p []byte) (n int, err error) {
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
	w.seq.Update(w.data, 0)
	return len(p), err
}

// ReadFrom transfers data from the reader into the buffer.
func (w *SeqWriter) ReadFrom(r io.Reader) (n int64, err error) {
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
				w.seq.Update(w.data, 0)
				return n, err
			}
			p = p[k:]
		}
		if len(w.data) == w.BufferSize {
			w.seq.Update(w.data, 0)
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

func (w *SeqWriter) Sequence(blk *Block, flags int) (n int, err error) {
	n, err = w.seq.Sequence(blk, flags)
	w.w += n
	return n, err
}

// errOutsideBuffer indicates that a position value points actually outside the
// buffer.
var errOutsideBuffer = errors.New("lz: pos outside of sequencer buffer")

// ReadByteAt returns the byte at the absolute position pos unless pos is outside of
// the data stored in window.
func (w *SeqWriter) ReadByteAt(pos int64) (c byte, err error) {
	pos -= w.start
	if !(0 <= pos && pos < int64(len(w.data))) {
		return 0, errOutsideBuffer
	}
	return w.data[pos], nil
}

// ReadAt allows to read data from the window directly.
func (w *SeqWriter) ReadAt(p []byte, pos int64) (n int, err error) {
	pos -= w.start
	if !(0 <= pos && pos < int64(len(w.data))) {
		return 0, errOutsideBuffer
	}
	n = copy(p, w.data[pos:])
	return n, nil
}
