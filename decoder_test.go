package lz

import (
	"encoding/json"
	"testing"

	"github.com/ulikunitz/opt"
)

func TestDecoderConfigJSON(t *testing.T) {
	tests := []DecoderConfig{
		{},
		{
			WindowSize: opt.Val(16),
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

		if cfg != cfgGot {
			t.Fatalf("config mismatch: got %+v, want %+v", cfgGot, cfg)
		}
	}
}
