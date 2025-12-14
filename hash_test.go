package lz

import (
	"encoding/json"
	"testing"
)

func TestHashOptionsJSON(t *testing.T) {
	hopts := &HashOptions{
		InputLen: 4,
		HashBits: 16,
	}

	data, err := json.Marshal(hopts)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	expected := `{"MapperType":"hash","InputLen":4,"HashBits":16}`
	if string(data) != expected {
		t.Errorf("json.Marshal = %s; want %s", data, expected)
	}

	var decoded HashOptions
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	if decoded.InputLen != hopts.InputLen {
		t.Errorf("json.Unmarshal InputLen = %d; want %d",
			decoded.InputLen, hopts.InputLen)
	}
	if decoded.HashBits != hopts.HashBits {
		t.Errorf("json.Unmarshal HashBits = %d; want %d",
			decoded.HashBits, hopts.HashBits)
	}
}
