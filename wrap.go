// SPDX-FileCopyrightText: © 2021 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

package lz

import (
	"io"
)

// Wrap combines a reader and a Parser and makes a Parser. The user
// doesn't need to take care of filling the Parser with additional data. The
// returned parser returns EOF if no further data is available.
//
// Wrap chooses the minimum of 32 kbyte or half of the window size as shrink
// size.
func Wrap(r io.Reader, seq Parser) *WrappedParser {
	return &WrappedParser{r: r, s: seq}
}

// WrappedParser is returned by the Wrap function. It provides the Parse
// method and reads the data required automatically from the stored reader.
type WrappedParser struct {
	r io.Reader
	s Parser
}

// Parse creates a block of sequences but reads the required data from the
// reader if necessary. The function returns io.EOF if no further data is
// available.
func (s *WrappedParser) Parse(blk *Block, flags int) (n int, err error) {
	for {
		n, err = s.s.Parse(blk, flags)
		if err != ErrEmptyBuffer {
			return n, err
		}
		s.s.Shrink()
		if k, err := s.s.ReadFrom(s.r); k == 0 {
			if err == ErrFullBuffer {
				panic("unexpected ErrFullBuffer")
			}
			return 0, err
		}
	}
}

// Reset puts the WrappedParser in its initial state and changes the wrapped
// reader to another reader.
func (s *WrappedParser) Reset(r io.Reader) {
	if err := s.s.Reset(nil); err != nil {
		panic(err)
	}
	s.r = r
}
