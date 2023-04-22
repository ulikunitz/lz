package suffix

import (
	"fmt"
	"math"
)

func segments(sa, lcp []int32, m int32, maxLen int32, f func(m int, s []int32)) {
	i := 0
	next := maxLen + 1
	new := false
	for j := 1; j <= len(lcp); j++ {
		if j < len(lcp) {
			n := lcp[j]
			switch {
			case n > m:
				if n < next {
					next = n
				}
				continue
			case n == m:
				new = true
				continue
			}
		}
		if j-i >= 2 {
			if next <= maxLen {
				// call scanLCP recursively
				segments(sa[i:j], lcp[i:j], next, maxLen, f)
			}
			if m == maxLen || new {
				// Inform caller
				f(int(m), sa[i:j])
			}
			new = false
		}
		i = j
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
