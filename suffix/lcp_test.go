package suffix

import (
	"bytes"
	"testing"
)

func FuzzLCP(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte("a"))
	f.Add([]byte("ab"))
	f.Add([]byte("ba"))
	f.Add([]byte("ababbab"))
	f.Fuzz(func(t *testing.T, p []byte) {
		sa := make([]int32, len(p))
		Sort(p, sa)
		if len(sa) > 0 {
			for i, j1 := range sa[:len(sa)-1] {
				j2 := sa[i+1]
				if bytes.Compare(p[j1:], p[j2:]) > 0 {
					t.Fatalf("p[sa[%d]=%d:] > p[sa[%d]=%d:]",
						i, i+1, j1, j2)
				}
			}
		}
		lcp := make([]int32, len(p))
		LCP(p, sa, nil, lcp)
		for i, l := range lcp {
			if i == 0 {
				if l != 0 {
					t.Fatal("lcp[0] != 0")
				}
				continue
			}
			n := matchLen(p[sa[i-1]:], p[sa[i]:])
			if n != int(l) {
				t.Fatalf("lcp[%d] = %d; want %d",
					i, l, n)
			}
		}
	})
}
