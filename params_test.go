package lz

import (
	"reflect"
	"testing"
)

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

// TODO: check the offsets
func memSize(c SeqConfig) int {
	switch p := c.(type) {
	case *DHSConfig:
		return p.WindowSize + (1<<p.HashBits1+1<<p.HashBits2)*8 + 207
	case *BDHSConfig:
		return p.WindowSize + (1<<p.HashBits1+1<<p.HashBits2)*8 + 207
	case *HSConfig:
		return p.WindowSize + (1<<p.HashBits)*8 + 161 - p.InputLen
	case *BHSConfig:
		return p.WindowSize + (1<<p.HashBits)*8 + 161 - p.InputLen
	default:
		panic("unexpected type")
	}
}

func TestComputeConfig(t *testing.T) {
	tests := []struct {
		p       Params
		seqType string
	}{
		{Params{}, "DHSConfig"},
		{Params{Effort: 1}, "HSConfig"},
		{Params{Effort: 9}, "BDHSConfig"},
		{Params{Effort: 1, MemoryBudget: 100 * kb}, "HSConfig"},
		{Params{Effort: 5, WindowSize: 64 * kb}, "DHSConfig"},
	}
	for _, tc := range tests {
		c, err := Config(tc.p)
		if err != nil {
			t.Fatalf("%+v.computeConfig() error %s", tc.p, err)
		}
		s := reflect.Indirect(reflect.ValueOf(c)).Type().Name()
		if s != tc.seqType {
			t.Fatalf("%+v: got type %s; want %s", tc.p, s,
				tc.seqType)
		}
		ms := memSize(c)
		var budget int
		if tc.p.MemoryBudget == 0 {
			i := tc.p.Effort
			if i == 0 {
				i = 5
			}
			budget = memoryBudgetTable[i]
		} else {
			budget = tc.p.MemoryBudget
		}
		if ms > budget {
			t.Fatalf("memSize: got %d; must be <= %d",
				ms, tc.p.MemoryBudget)
		}
	}
}
