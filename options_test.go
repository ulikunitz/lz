package lz

import (
	"encoding/json"
	"testing"
)

func TestParserOptionsJSON(t *testing.T) {
	opts := ParserOptions{
		Parser:     Greedy,
		BlockSize:  65536,
		WindowSize: 32768,
		BufferSize: 4096,
	}
	data, err := json.MarshalIndent(opts, "", "  ")
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	t.Logf("Marshalled JSON:\n%s", data)

	var optsG ParserOptions
	if err := json.Unmarshal(data, &optsG); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if optsG != opts {
		t.Fatalf("json.Unmarshal: got %+v; want %+v", optsG, opts)
	}
}
