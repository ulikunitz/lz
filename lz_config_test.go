package lz

import "testing"

func TestFindHSParams(t *testing.T) {
	tests := []struct {
		m int
		p hsParams
	}{
		{-1, hsParams{64 * kb, 3, 11}},
		{128 * kb, hsParams{128 * kb, 3, 13}},
		{548 * kb, hsParams{384 * kb, 3, 15}},
		{50176 * kb, hsParams{50176 * kb, 5, 22}},
		{100000 * kb, hsParams{50176 * kb, 5, 22}},
	}
	for _, tc := range tests {
		q := findHSParams(hsParameters, tc.m)
		if q != tc.p {
			t.Fatalf("findHSParams(hsParameters, %d) got %+v; "+
				" want %+v", tc.m, q, tc.p)
		}
	}
}

func TestFindDHSParams(t *testing.T) {
	tests := []struct {
		m int
		p dhsParams
	}{
		{m: -1, p: dhsParams{64 * kb, 2, 10, 4, 11}},
		{m: 128 * kb, p: dhsParams{128 * kb, 2, 11, 5, 13}},
		{m: 548 * kb, p: dhsParams{512 * kb, 3, 15, 6, 14}},
		{m: 50176 * kb, p: dhsParams{27684 * kb, 3, 16, 8, 21}},
		{m: 100000 * kb, p: dhsParams{65536 * kb, 3, 16, 8, 22}},
	}
	for _, tc := range tests {
		q := findDHSParams(dhsParameters, tc.m)
		if q != tc.p {
			t.Fatalf("findDHSParams(dhsParameters, %d) got %+v; "+
				" want %+v", tc.m, q, tc.p)
		}
	}
}
