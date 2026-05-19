package jsontypes

import (
	"encoding/json"
	"testing"
)

func TestSizeSimpleJSON(t *testing.T) {
	tests := []struct {
		s Size
	}{
		{0},
		{128},
		{8 << 20},
		{16 << 10},
		{2 << 30},
	}

	for _, tc := range tests {
		t.Run(tc.s.String(), func(t *testing.T) {
			data, err := json.Marshal(tc.s)
			if err != nil {
				t.Fatalf("json.Marshal: unexpected error: %v",
					err)
			}
			var s Size
			err = json.Unmarshal(data, &s)
			if err != nil {
				t.Fatalf("json.Unmarshal: unexpected error: %v",
					err)
			}
			if s != tc.s {
				t.Fatalf("unexpected size: got %d, want %d",
					s, tc.s)
			}
		})
	}
}
