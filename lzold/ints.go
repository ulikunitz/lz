// SPDX-FileCopyrightText: © 2021 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

package lzold

const (
	intSize   = 32 << (^uint(0) >> 63)
	maxInt    = 1<<(intSize-1) - 1
	maxUint32 = 1<<32 - 1
)

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

func doz(x, y int) int {
	return (x - y) & (-iverson(x >= y))
}

func min(x, y int) int {
	return x - doz(x, y)
}
