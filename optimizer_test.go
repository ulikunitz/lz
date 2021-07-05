package lz

import (
	"fmt"
	"testing"
)

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

func TestReduceMatches(t *testing.T) {
	tests := []struct {
		in  []match
		n   int
		out []match
	}{
		{
			in: []match{{8, 15, 3}, {2, 4, 1}, {4, 6, 1},
				{3, 5, 1}, {3, 6, 2}, {7, 14, 3}},
			n:   20,
			out: []match{{2, 6, 1}, {7, 15, 3}},
		},
		{
			in:  []match{{2, 4, 1}, {5, 9, 2}, {3, 5, 3}},
			n:   20,
			out: []match{{2, 4, 1}, {3, 5, 3}, {5, 9, 2}},
		},
	}

	for i, tc := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			in := make([]match, len(tc.in))
			copy(in, tc.in)
			out := reduceMatches(in, tc.n)
			if !equalMatches(out, tc.out) {
				t.Logf("in: %+v", tc.in)
				t.Logf("out: %+v", out)
				t.Fatalf("want %+v", tc.out)
			}
		})
	}

}
