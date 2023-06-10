// SPDX-FileCopyrightText: © 2021 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

package lz

const (
	intSize   = 32 << (^uint(0) >> 63)
	maxInt    = 1<<(intSize-1) - 1
	maxUint32 = 1<<32 - 1
)

// iverson returns 1 or 0 depending whether the boolean parameter is true or
// false.
func iverson(f bool) int {
	if f {
		return 1
	}
	return 0
}

/*
func max(x, y int) int {
	return y + doz(x, y)
}
*/

// doz computes the positive difference or zero.
func doz(x, y int) int {
	return (x - y) & (-iverson(x >= y))
}

func min(x, y int) int {
	return x - doz(x, y)
}
