// SPDX-FileCopyrightText: © 2021 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

package suffix

import (
	"fmt"
	"math"
)

func scanLCP(sa, lcp []int32, minLen, maxLen int32, f func(m int, s []int32)) {
	type item struct {
		n int32
		j int32
	}
	stack := make([]item, 1, 16)
	// stack[1] = item{0, 0} -- make function in line above set item[0] to
	// zero implicitly
scan:
	for j := int32(1); ; j++ {
		var n int32
		if j < int32(len(lcp)) {
			n = lcp[j]
			if n > maxLen {
				n = maxLen
			}
		} else {
			n = -1
		}
		for {
			top := stack[len(stack)-1]
			switch {
			case n > top.n:
				stack = append(stack, item{n, j - 1})
				continue scan
			case n == top.n:
				continue scan
			}
			if top.n >= minLen {
				f(int(top.n), sa[top.j:j])
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				break scan
			}
		}
	}
}

// Segments returns all segments of suffixes that share common prefixes of
// length n. The segments can be sorted or permuted in any way. The suffix array
// sa will be modified. As a consequence the segments can be in any order.
func Segments(sa, lcp []int32, minLen, maxLen int, f func(m int, segment []int32)) {
	if len(sa) != len(lcp) {
		panic(fmt.Errorf("len(sa)=%d != len(lcp)=%d", sa, lcp))
	}
	if !(0 <= minLen && minLen <= math.MaxInt32) {
		panic(fmt.Errorf("minLen=%d out of range", minLen))
	}
	if !(maxLen <= math.MaxInt32) {
		panic(fmt.Errorf("maxLen=%d larger than MaxInt32=%d",
			maxLen, math.MaxInt32))
	}
	if maxLen < minLen {
		return
	}
	scanLCP(sa, lcp, int32(minLen), int32(maxLen), f)
}
