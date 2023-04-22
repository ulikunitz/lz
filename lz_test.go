package lz

import (
	"bytes"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func testSequencer(t *testing.T, cfg SeqConfig, p []byte) {
	cfg.ApplyDefaults()
	t.Logf("cfg.ApplyDefaults() %+v", cfg)
	if err := cfg.Verify(); err != nil {
		t.Skip()
	}
	bcfg := cfg.BufferConfig()

	seq, err := cfg.NewSequencer()
	if err != nil {
		t.Fatalf("cfg.NewSequencer() error %s", err)
	}
	s := Wrap(bytes.NewReader(p), seq)

	var buffer bytes.Buffer
	var decoder Decoder
	err = decoder.Init(&buffer, DConfig{WindowSize: bcfg.WindowSize})
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
}

func FuzzBHS(f *testing.F) {
	f.Add(3, 5, []byte("=====foofoobarfoobar bartender===="))
	f.Fuzz(func(t *testing.T, inputLen int, hashBits int, p []byte) {
		cfg := &BHSConfig{
			BufConfig{
				WindowSize: 1024,
				BlockSize:  512,
			},
			HashConfig{
				InputLen: inputLen,
				HashBits: hashBits,
			},
		}
		testSequencer(t, cfg, p)
	})
}

func FuzzDHS(f *testing.F) {
	f.Add(3, 5, 4, 6, []byte("=====foofoobarfoobar bartender===="))
	f.Fuzz(func(t *testing.T,
		inputLen1, hashBits1 int,
		inputLen2, hashBits2 int,
		p []byte) {

		cfg := &DHSConfig{
			BufConfig{
				WindowSize: 1024,
				BlockSize:  512,
			},
			DHConfig{
				H1cfg: HashConfig{inputLen1, hashBits1},
				H2cfg: HashConfig{inputLen2, hashBits2},
			},
		}
		testSequencer(t, cfg, p)
	})
}

func FuzzBDHS(f *testing.F) {
	f.Add(3, 5, 4, 6, []byte("=====foofoobarfoobar bartender===="))
	f.Fuzz(func(t *testing.T,
		inputLen1, hashBits1 int,
		inputLen2, hashBits2 int,
		p []byte) {

		cfg := &BDHSConfig{
			BufConfig{
				WindowSize: 1024,
				BlockSize:  512,
			},
			DHConfig{
				H1cfg: HashConfig{inputLen1, hashBits1},
				H2cfg: HashConfig{inputLen2, hashBits2},
			},
		}
		testSequencer(t, cfg, p)
	})
}

func FuzzBUHS(f *testing.F) {
	f.Add(3, 5, 8, []byte("=====foofoobarfoobar bartender===="))
	f.Fuzz(func(t *testing.T,
		inputLen, hashBits, bucketSize int,
		p []byte) {

		cfg := &BUHSConfig{
			BufConfig{
				WindowSize: 1024,
				BlockSize:  512,
			},
			BUHConfig{
				InputLen:   inputLen,
				HashBits:   hashBits,
				BucketSize: bucketSize,
			},
		}
		cfg.ApplyDefaults()
		// We need to limit the memory consumption for Fuzzing.
		if cfg.HashBits > 21 {
			t.Skip()
		}
		testSequencer(t, cfg, p)
	})
}

func FuzzGSAS(f *testing.F) {
	f.Add([]byte("=====foofoobarfoobar bartender===="))
	f.Fuzz(func(t *testing.T, p []byte) {
		cfg := &GSASConfig{
			BufConfig: BufConfig{
				WindowSize: 1024,
				BlockSize:  512,
			},
		}
		testSequencer(t, cfg, p)
	})
}
