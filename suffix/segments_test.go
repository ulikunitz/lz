package suffix

import (
	"sort"
	"testing"
)

func logSALCP(t *testing.T, p []byte, sa, lcp []int32) {
	for j, i := range sa {
		t.Logf("%3d %3d %s", i, lcp[j], p[i:])
	}

}

func logSuffixes(t *testing.T, p []byte, s []int32) {
	for _, i := range s {
		t.Logf("%3d %s", i, p[i:])
	}
}

func TestSegments(t *testing.T) {
	tests := []string{
		"abbababb",
		"mississippi",
		"=====foofoobarfoobar bartender====",
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc, func(t *testing.T) {
			p := []byte(tc)
			sa := make([]int32, len(tc))
			Sort(p, sa)
			lcp := make([]int32, len(p))
			LCP(p, sa, nil, lcp)
			t.Log("## SuffixArray")
			logSALCP(t, p, sa, lcp)
			Segments(sa, lcp, 2, 10, func(n int, s []int32) {
				t.Logf("## Segment n=%d", n)
				sort.SliceStable(s, func(i, j int) bool {
					return s[i] < s[j]
				})
				logSuffixes(t, p, s)
			})
		})

	}

}
