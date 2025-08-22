// SPDX-FileCopyrightText: © 2021 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

package lz

// Kilobytes and Megabyte defined as the more precise kibibyte and mebibyte.
const (
	kiB = 1 << 10
	miB = 1 << 20
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
