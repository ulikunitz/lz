package suffix

import (
	"fmt"
	"math"
	"math/bits"
)

// LCPExt computes the LCP table using the original text t, the suffix array sa
// and the inverse of it sainv.
func LCPExt(t []byte, sa []int32, sainv []int32, lcp []int32) {
	if len(t) > math.MaxInt32 {
		panic(fmt.Errorf("suffix: len(t)=%d > MaxInt32", len(t)))
	}
	if !(len(t) == len(sa) && len(t) == len(lcp) && len(t) == len(sainv)) {
		panic(fmt.Errorf("suffix: all slices must have same lengths"))
	}

	_LCPExt(t, sa, sainv, lcp)
}

// _LCPExt provides actual functionality without the error checks.
//
// The algorithm used uses the phi function and the theorem regarding it.
func _LCPExt(t []byte, sa []int32, sainv []int32, lcp []int32) {
	l := int32(0)
	for i, k := range sainv {
		if k == 0 {
			lcp[0] = 0
			l = 0
			continue
		}
		j := sa[k-1] // j = phi(i)
		l += int32(matchLen(t[int32(i)+l:], t[j+l:]))
		lcp[k] = l
		if l > 0 {
			l--
		}
	}
}

// LCP computes the LCP table for t. Note the sainv array will be temporarily
// computed.
func LCP(t []byte, sa []int32, lcp []int32) {
	if len(t) > math.MaxInt32 {
		panic(fmt.Errorf("suffix: len(t)=%d > MaxInt32", len(t)))
	}
	if !(len(t) == len(sa) && len(t) == len(lcp)) {
		panic(fmt.Errorf("suffix: all slices must have same lengths"))
	}
	sainv := make([]int32, len(sa))
	for i, j := range sa {
		sainv[j] = int32(i)
	}
	_LCPExt(t, sa, sainv, lcp)
}

// matchLen computes the length of the common prefix between p and q.
func matchLen(p, q []byte) int {
	if len(q) > len(p) {
		p, q = q, p
	}
	n := 0
	for len(q) >= 8 {
		x := _getLE64(p) ^ _getLE64(q)
		k := bits.TrailingZeros64(x) >> 3
		n += k
		if k < 8 {
			return n
		}
		q = q[8:]
		p = p[8:]
	}
	if len(q) >= 4 {
		x := _getLE32(p) ^ _getLE32(q)
		k := bits.TrailingZeros32(x) >> 3
		n += k
		if k < 4 {
			return n
		}
		q = q[4:]
		p = p[4:]
	}
	for i, b := range q {
		if p[i] != b {
			break
		}
		n++
	}
	return n
}

// _getLE64 loads a uint64 value from the p field. This function will be inlined
// and compiled into a simple move on little-endian 64 bit architectures.
//
// If p is too small the function will panic.
func _getLE64(p []byte) uint64 {
	_ = p[7]
	return uint64(p[0]) | uint64(p[1])<<8 | uint64(p[2])<<16 |
		uint64(p[3])<<24 | uint64(p[4])<<32 | uint64(p[5])<<40 |
		uint64(p[6])<<48 | uint64(p[7])<<56
}

// _getLE32 loads a uint32 value from the p field. This function will be inlined
// and compiled into a simple move on little-endian architectures.
//
// If p is too small the function will panic.
func _getLE32(p []byte) uint32 {
	_ = p[3]
	return uint32(p[0]) | uint32(p[1])<<8 | uint32(p[2])<<16 |
		uint32(p[3])<<24
}
