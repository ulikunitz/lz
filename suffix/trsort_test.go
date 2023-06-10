// SPDX-FileCopyrightText: © 2021 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

package suffix

import (
	"testing"
)

func equalInt32s(a, b []int32) bool {
	if len(a) != len(b) {
		return false
	}
	for i, x := range a {
		if x != b[i] {
			return false
		}
	}
	return true
}

func TestTRSort(t *testing.T) {
	tests := []struct {
		a, r   []int32
		result []int32
	}{
		{
			a:      []int32{-1, 2, 1, 0, -1},
			r:      []int32{3, 3, 3, 0, 4},
			result: []int32{3, 2, 1, 0, 4},
		},
		{
			a:      []int32{-4, 8, 7, 0, 10, 2, 9, 1, -4, 11, 3, 5},
			r:      []int32{3, 7, 5, 10, 0, 11, 8, 2, 1, 7, 5, 9},
			result: []int32{3, 7, 5, 10, 0, 11, 8, 2, 1, 6, 4, 9},
		},
	}

	for _, tc := range tests {
		config{}.trSort(tc.a, tc.r)
		if !equalInt32s(tc.result, tc.r) {
			t.Fatalf("trSort returned %d; wanted %d", tc.r,
				tc.result)
		}
	}
}

func TestABAC(t *testing.T) {
	sa := []int32{0, 6, 5, 4, 3, 2, 1, -2, 8}
	isa := []int32{6, 6, 6, 6, 6, 6, 6, 7, 8}

	config{}.trSort(sa, isa)

	if len(isa) != 9 {
		t.Fatalf("len(isa) is %d; want %d", len(isa), 9)
	}

	for i, k := range isa {
		if int32(i) != k {
			t.Errorf("i=%d != k=%d", i, k)
		}
	}
}

func TestABBA(t *testing.T) {
	sa := []int32{-1, 2, 1, 0}
	isa := []int32{3, 3, 3, 0}
	isaWant := []int32{3, 2, 1, 0}

	config{}.trSort(sa, isa)

	if len(isa) != len(sa) {
		t.Fatalf("len(isa) is %d; want %d", len(isa), len(sa))
	}

	for i, k := range isa {
		if k != isaWant[i] {
			t.Errorf("isa[%d]=%d; want %d", i, k, isaWant[i])
		}
	}
}

func TestTRInsertionSort(t *testing.T) {
	tests := []struct{ sa, isaD, saWant []int32 }{
		{sa: []int32{1, 2, 0}, isaD: []int32{3, 3, 0},
			saWant: []int32{2, -2, 0}},
		{sa: []int32{0, 6, 5, 4, 3, 2, 1},
			isaD:   []int32{6, 6, 6, 6, 6, 6, 7},
			saWant: []int32{-1, -6, -5, -4, -3, 1, 6},
		},
	}
	for _, tc := range tests {
		sa := make([]int32, len(tc.sa))
		copy(sa, tc.sa)
		trInsertionSort(sa, tc.isaD)
		if !equalInt32s(sa, tc.saWant) {
			t.Fatalf("sa=%d; want %d; isaD=%d", sa, tc.saWant,
				tc.isaD)
		}
	}
}

func TestTRHeapSort(t *testing.T) {
	tests := []struct{ sa, isaD, saWant []int32 }{
		{sa: []int32{1, 2, 0}, isaD: []int32{3, 3, 0},
			saWant: []int32{2, -1, 1},
		},
		{sa: []int32{0, 6, 5, 4, 3, 2, 1},
			isaD:   []int32{6, 6, 6, 6, 6, 6, 7},
			saWant: []int32{-1, -6, -5, -4, -3, 1, 6},
		},
	}
	for _, tc := range tests {
		sa := make([]int32, len(tc.sa))
		copy(sa, tc.sa)
		trHeapSort(sa, tc.isaD)
		if !equalInt32s(sa, tc.saWant) {
			t.Fatalf("sa=%d; want %d; isaD=%d", sa, tc.saWant,
				tc.isaD)
		}
	}
}
