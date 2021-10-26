package lz

import "testing"

func TestFindHSParams(t *testing.T) {
	tests := []struct {
		m int
		p hsParams
	}{
		// {-1, hsParams{64 * kb, 3, 11}},
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
