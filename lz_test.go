package lz

import (
	"encoding/json"
	"testing"
)

func TestParserOptions(t *testing.T) {
	tests := []struct {
		opts ParserOptions
	}{
		{ParserOptions{}},
		{ParserOptions{
			PathFinder:    "greedy",
			Mapper:        "hash_4:16",
			WindowSize:    64 << 10,
			RetentionSize: 16 << 10,
			BufferSize:    64 << 10,
		}},
		{ParserOptions{
			PathFinder:    "greedy",
			Mapper:        "hash_4:16",
			WindowSize:    2000,
			RetentionSize: 1000,
			BufferSize:    4000,
			MinMatchLen:   4,
			MaxMatchLen:   64,
		}},
	}
	for _, tc := range tests {
		data, err := json.Marshal(&tc.opts)
		if err != nil {
			t.Errorf("json.Marshal(%v) returned unexpected error: %s",
				tc.opts, err)
			continue
		}
		t.Logf("# json.Marshal(%v) = %s", tc.opts, data)
		var opts ParserOptions
		err = json.Unmarshal(data, &opts)
		if err != nil {
			t.Errorf("json.Unmarshal(%s) returned unexpected error: %s",
				data, err)
			continue
		}
		if opts != tc.opts {
			t.Errorf("json.Unmarshal(%s) = %v; want %v",
				data, opts, tc.opts)
		}
	}
}
