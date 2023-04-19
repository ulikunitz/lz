package lz

import "testing"

func TestBackwardMatchLen(t *testing.T) {
	tests := []struct {
		p, q []byte
		n    int
	}{
		{p: []byte("hello"), q: []byte("xxxhello"), n: 5},
		{p: []byte("foohello"), q: []byte("arhello"), n: 5},
		{p: nil, q: []byte("foo"), n: 0},
		{p: nil, q: nil, n: 0},
		{p: []byte("12345foofoobar"), q: []byte("abcfoofoobar"), n: 9},
		{p: []byte("foobarfoobar"), q: []byte("foobarfoobar"), n: 12},
		{p: []byte("foo"), q: []byte("bar"), n: 0},
	}
	for _, tc := range tests {
		n := lcs(tc.p, tc.q)
		if n != tc.n {
			t.Fatalf("backwardMatchLen(%q, %q) is %d; want %d",
				tc.p, tc.q, n, tc.n)
		}
	}
}

func simpleLCP(p, q []byte) int {
	if len(p) < len(q) {
		p, q = q, p
	}
	n := 0
	for i, b := range q {
		if p[i] != b {
			break
		}
		n++
	}
	return n
}

func FuzzLCP(f *testing.F) {
	f.Add([]byte("Hello, universe!"), []byte("Hello, world!"))
	f.Add([]byte(""), []byte("abc"))
	f.Add([]byte(""), []byte(""))
	f.Fuzz(func(t *testing.T, p, q []byte) {
		g := lcp(p, q)
		w := simpleLCP(p, q)
		if g != w {
			t.Fatalf("lpc(%q, %q) = %d; want %d", p, q, g, w)
		}
	})
}
