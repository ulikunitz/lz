package lz

import "testing"

func TestParseHashName(t *testing.T) {
	tests := []struct {
		name     string
		inputLen int
		hashBits int
		wrong    bool
	}{
		{"hash_2:0", 2, 0, false},
		{"hash_2:1", 2, 1, false},
		{"hash_2:16", 2, 16, false},
		{"hash_2:17", 2, 17, true},
		{"hash_4:20", 4, 20, false},
		{"h_4:21", 0, 0, true},
		{"hash_-4:-2", 0, 0, true},
		{"hash_2:10xxx", 0, 0, true},
		{"hash-2:16", 0, 0, true},
	}
	for _, tc := range tests {
		inputLen, hashBits, err := parseHashName(tc.name)
		if tc.wrong {
			if err == nil {
				t.Errorf(
					"parseHashName(%q) = %d, %d, nil; want non-nil error",
					tc.name, inputLen, hashBits)
				continue
			}
			t.Logf("# parseHashName(%q) returned expected error: %s",
				tc.name, err)
			continue
		}
		if err != nil {
			t.Errorf("parseHashName(%q) returned unexpected error: %s",
				tc.name, err)
			continue
		}
		if inputLen != tc.inputLen || hashBits != tc.hashBits {
			t.Errorf(
				"parseHashName(%q) = %d, %d, nil; want inputLen=%d, hashBits=%d",
				tc.name, inputLen, hashBits,
				tc.inputLen, tc.hashBits)
		}
	}
}
