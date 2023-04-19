package lz

import (
	"math/bits"
)

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

// getLE64 reads the 64-bit little-endian representation independent of the
// length of slice p.
func getLE64(p []byte) uint64 {
	switch len(p) {
	case 0:
		return 0
	case 1:
		return uint64(p[0])
	case 2:
		_ = p[1]
		return uint64(p[0]) | uint64(p[1])<<8
	case 3:
		_ = p[2]
		return uint64(p[0]) | uint64(p[1])<<8 | uint64(p[2])<<16
	case 4:
		return uint64(_getLE32(p))
	case 5:
		_ = p[4]
		return uint64(_getLE32(p)) | uint64(p[4])<<32
	case 6:
		_ = p[5]
		return uint64(_getLE32(p)) | uint64(p[4])<<32 |
			uint64(p[5])<<40
	case 7:
		_ = p[6]
		return uint64(_getLE32(p)) | uint64(p[4])<<32 |
			uint64(p[5])<<40 | uint64(p[6])<<48
	default:
		return _getLE64(p)
	}
}

// lcp computes the length of the longest common prefix between p and q.
func lcp(p, q []byte) int {
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

// lcs computes the longest common suffix
func lcs(p, q []byte) int {
	if len(q) > len(p) {
		p, q = q, p
	}
	p = p[len(p)-len(q):]
	n := 0
	var i int
	for i = len(q) - 8; i >= 0; i -= 8 {
		x := _getLE64(p[i:]) ^ _getLE64(q[i:])
		k := bits.LeadingZeros64(x) >> 3
		n += k
		if k < 8 {
			return n
		}
	}
	i += 8
	if i > 0 {
		s := (8 - i) << 3
		x := getLE64(q) << s
		x ^= getLE64(p) << s
		k := bits.LeadingZeros64(x) >> 3
		if k > i {
			k = i
		}
		n += k
	}
	return n
}
