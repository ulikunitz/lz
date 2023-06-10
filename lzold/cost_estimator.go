// SPDX-FileCopyrightText: © 2021 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

package lzold

import "math/bits"

// CostEstimator provides a cost estimation to encode matches and literals. The
// costs are provided for a match with a non-zero offset or m literal bytes with
// a zero o value. The Costs should be provided in bits, but other measures like
// 1/100th of a bit are also possible. The Update method is provided to update
// the offset history.
type CostEstimator interface {
	Cost(m, o uint32) uint64
	Push(o uint32)
	Reset()
}

// SimpleEstimator provides a very simple cost model for compression. It
// supports offset repeats as in LZMA.
type SimpleEstimator struct {
	Rep [4]uint32
}

func (e *SimpleEstimator) Reset() {
	for i := range e.Rep {
		e.Rep[i] = 0
	}
}

// Push writes the offset o into the history.
func (e *SimpleEstimator) Push(o uint32) {
	r := &e.Rep
	switch o {
	default:
		r[3] = r[2]
		fallthrough
	case r[2]:
		r[2] = r[1]
		fallthrough
	case r[1]:
		r[1] = r[0]
		r[0] = o
	case r[0]:
	}
}

// Cost provides a simple cost estimation for the match or a literal, offset 0.
func (e *SimpleEstimator) Cost(m, o uint32) uint64 {
	if o == 0 {
		return 8 * uint64(m)
	}
	g := 0
	for ; g < len(e.Rep); g++ {
		if e.Rep[g] == o {
			break
		}
	}
	c := uint64(1 + bits.Len32(m))
	if g >= 4 {
		c += uint64(bits.Len32(o))
	}
	return c
}
