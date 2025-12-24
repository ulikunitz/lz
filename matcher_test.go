package lz

import (
	"encoding/json"
	"testing"
)

func TestGenericMatcherOptionsJSON(t *testing.T) {
	origOpts := &GenericMatcherOptions{
		MinMatchLen:   4,
		MaxMatchLen:   273,
		MapperOptions: &HashOptions{
			InputLen: 3,
			HashBits: 17,
		},
	}

	data, err := json.Marshal(origOpts)
	if err != nil {
		t.Fatalf("MarshalJSON error: %v", err)
	}

	t.Logf("Marshaled JSON: %s", data)

	m, err := UnmarshalJSONMatcherOptions(data)
	if err != nil {
		t.Fatalf("UnmarshalJSONMatcherOptions error: %v", err)
	}

	opts, ok := m.(*GenericMatcherOptions)
	if !ok {
		t.Fatalf("Unmarshaled matcher options has wrong type: %T", m)
	}

	if opts.MinMatchLen != origOpts.MinMatchLen {
		t.Errorf("MinMatchLen: got %d, want %d",
			opts.MinMatchLen, origOpts.MinMatchLen)
	}
	if opts.MaxMatchLen != origOpts.MaxMatchLen {
		t.Errorf("MaxMatchLen: got %d, want %d",
			opts.MaxMatchLen, origOpts.MaxMatchLen)
	}

	hashOpts, ok := opts.MapperOptions.(*HashOptions)
	if !ok {
		t.Fatalf("Unmarshaled mapper options has wrong type: %T",
			opts.MapperOptions)
	}
	if hashOpts.InputLen != 3 {
		t.Errorf("MapperOptions.InputLen: got %d, want %d",
			hashOpts.InputLen, 3)
	}
	if hashOpts.HashBits != 17 {
		t.Errorf("MapperOptions.HashBits: got %d, want %d",
			hashOpts.HashBits, 17)
	}
}
