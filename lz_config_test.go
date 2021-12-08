package lz

import "testing"

func TestWindowHS(t *testing.T) {
	tests := []struct {
		winSize    int
		winSizeOut int
		n          int
	}{
		{1313, 32768, 10},
		{32768, 32768, 10},
		{65536, 65536, 10},
		{65537, 65536, 10},
		{6291456, 6291456, 15},
		{6291457, 6291456, 15},
	}
	for _, tc := range tests {
		s := windowHS(hsWinParameters, tc.winSize)
		if len(s) != tc.n {
			t.Fatalf("winSize=%d: want %d entries; got %d",
				tc.winSize, tc.n, len(s))
		}
		for i, p := range s {
			if p.size != tc.winSizeOut {
				t.Fatalf("s[%d]: want size=%d; got %d",
					i, tc.winSizeOut, p.size)
			}
		}
	}
}

func TestSimpleComputeConfig(t *testing.T) {
	cfg := Config{MemoryBudget: 8 << 20}
	c, err := cfg.computeConfig()
	if err != nil {
		t.Fatalf("config.computeConfg() error %s", err)
	}
	t.Logf("c %#v", c)
}
