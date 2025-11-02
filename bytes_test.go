package lz

import "testing"

func TestLCP4(t *testing.T) {
	a := []byte("123455681234")
	b := []byte("12345568123457")

	n := lcp(a, b)
	if n != len(a) {
		t.Fatalf("expected %d, got %d", len(a), n)
	}
}
