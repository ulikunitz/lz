// SPDX-FileCopyrightText: © 2021 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

package lz

import (
	"io"
)

// The code below supports the testing.

// wrap combines a reader and a Parser and makes a Parser. The user
// doesn't need to take care of filling the Parser with additional data. The
// returned parser returns EOF if no further data is available.
func wrap(r io.Reader, p Parser) *wrappedParser {
	return &wrappedParser{r: r, p: p}
}

// wrappedParser is returned by the Wrap function. It provides the Parse
// method and reads the data required automatically from the stored reader.
type wrappedParser struct {
	r io.Reader
	p Parser
}

// Parse creates a block of sequences but reads the required data from the
// reader if necessary. The function returns io.EOF if no further data is
// available.
func (wp *wrappedParser) Parse(blk *Block, flags int) (n int, err error) {
	for {
		n, err = wp.p.Parse(blk, flags)
		if err != ErrEmptyBuffer {
			return n, err
		}
		wp.p.Shrink()
		if k, err := wp.p.ReadFrom(wp.r); k == 0 {
			if err == ErrFullBuffer {
				panic("unexpected ErrFullBuffer")
			}
			return 0, err
		}
	}
}

// Reset puts the WrappedParser in its initial state and changes the wrapped
// reader to another reader.
func (wp *wrappedParser) Reset(r io.Reader) {
	if err := wp.p.Reset(nil); err != nil {
		panic(err)
	}
	wp.r = r
}
