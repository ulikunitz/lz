package lz

import "testing"

func equalMatches(a, b []match) bool {
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

func TestMergeMatches(t *testing.T) {
	tests := []struct {
		a, b, c []match
	}{
		{
			a: []match{{20, 10}, {19, 8}},
			b: []match{{20, 11}, {12, 3}},
			c: []match{{20, 10}, {19, 8}, {12, 3}},
		},
		{
			a: []match{{20, 10}},
			b: []match{{20, 10}},
			c: []match{{20, 10}},
		},
	}

	for _, tc := range tests {
		c := mergeMatches(nil, tc.a, tc.b)
		if !equalMatches(c, tc.c) {
			t.Fatalf("mergeMatches(%+v, %+v) got %+v; want %+v",
				tc.a, tc.b, c, tc.c)
		}
	}
}
