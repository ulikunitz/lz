package lz

import (
	"bytes"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func FuzzBackwardHashSequencer(f *testing.F) {
	f.Add(3, 5, []byte("=====foofoobarfoobar bartender===="))
	f.Fuzz(func(t *testing.T, inputLen int, hashBits int, p []byte) {
		cfg := BHSConfig{
			BufConfig{
				WindowSize: 1024,
				BlockSize:  512,
			},
			HashConfig{
				InputLen: inputLen,
				HashBits: hashBits,
			},
		}
		cfg.ApplyDefaults()
		t.Logf("cfg.ApplyDefaults() %+v", cfg)
		if err := cfg.Verify(); err != nil {
			t.Skip()
		}

		seq, err := cfg.NewSequencer()
		if err != nil {
			t.Fatalf("cfg.NewSequencer() error %s", err)
		}
		s := Wrap(bytes.NewReader(p), seq)

		var buffer bytes.Buffer
		var decoder Decoder
		err = decoder.Init(&buffer, DConfig{WindowSize: cfg.WindowSize})
		if err != nil {
			t.Fatalf("decoder.Init error %s", err)
		}

		var blk Block
		for {
			if _, err := s.Sequence(&blk, 0); err != nil {
				if err == io.EOF {
					break
				}
				t.Fatalf("s.Sequence error %s", err)
			}
			if _, _, _, err := decoder.WriteBlock(blk); err != nil {
				t.Fatalf("decoder.WriteBlock error %s", err)
			}
		}
		if err := decoder.Flush(); err != nil {
			t.Fatalf("decoder.Flush error %s", err)
		}

		q := buffer.Bytes()
		if diff := cmp.Diff(p, q, cmpopts.EquateEmpty()); diff != "" {
			t.Fatalf("decoded mismatch (+got -want):\n%s", diff)
		}
	})
}
