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

func memSize(c OldConfigurator) int {
	switch p := c.(type) {
	case *DHSConfig:
		return p.WindowSize + (1<<p.HashBits1+1<<p.HashBits2)*8 + 207
	case *BDHSConfig:
		return p.WindowSize + (1<<p.HashBits1+1<<p.HashBits2)*8 + 207
	case *OHSConfig:
		return p.WindowSize + (1<<p.HashBits)*8 + 161 - p.InputLen
	case *BHSConfig:
		return p.WindowSize + (1<<p.HashBits)*8 + 161 - p.InputLen
	default:
		panic("unexpected type")
	}
}

func TestComputeConfig(t *testing.T) {
	tests := []struct {
		cfg     Config
		seqType string
	}{
		{Config{}, "DHSConfig"},
		{Config{Effort: 1}, "OHSConfig"},
		{Config{Effort: 9}, "BDHSConfig"},
		{Config{Effort: 1, MemoryBudget: 100 * kb}, "OHSConfig"},
		{Config{Effort: 5, WindowSize: 64 * kb}, "DHSConfig"},
	}
	for _, tc := range tests {
		c, err := tc.cfg.computeConfig()
		if err != nil {
			t.Fatalf("%+v.computeConfig() error %s", tc.cfg, err)
		}
		s := reflect.Indirect(reflect.ValueOf(c)).Type().Name()
		if s != tc.seqType {
			t.Fatalf("%+v: got type %s; want %s", tc.cfg, s,
				tc.seqType)
		}
		ms := memSize(c)
		var budget int
		if tc.cfg.MemoryBudget == 0 {
			i := tc.cfg.Effort
			if i == 0 {
				i = 5
			}
			budget = memoryBudgetTable[i]
		} else {
			budget = tc.cfg.MemoryBudget
		}
		if ms > budget {
			t.Fatalf("memSize: got %d; must be <= %d",
				ms, tc.cfg.MemoryBudget)
		}
	}
}
