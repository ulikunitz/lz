package lz

import (
	"encoding/json"
	"testing"
)

func equalDecoderConfig(a, b DecoderConfig) bool {
	if a.BufferSize != b.BufferSize {
		return false
	}
	if a.WindowSize == nil || b.WindowSize == nil {
		return a.WindowSize == b.WindowSize
	}
	return *a.WindowSize == *b.WindowSize
}

func TestDecoderConfigJSON(t *testing.T) {
	tests := []DecoderConfig{
		{},
		{
			WindowSize: new(16),
			BufferSize: 32,
		},
		{
			BufferSize: 8 << 10,
		},
	}

	for _, cfg := range tests {
		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			t.Fatalf("json.MarshalIndent: %v", err)
		}
		t.Logf("config JSON:\n%s", data)
		var cfgGot DecoderConfig
		err = json.Unmarshal(data, &cfgGot)
		if err != nil {
			t.Fatalf("json.Unmarshal: %v", err)
		}

		if !equalDecoderConfig(cfgGot, cfg) {
			t.Fatalf("config mismatch: got %+v, want %+v", cfgGot, cfg)
		}
	}
}
