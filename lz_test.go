package lz

import (
	"bytes"
	"crypto/sha256"
	"io"
	"os"
	"testing"

	"github.com/ulikunitz/opt"
)

func FuzzConfig(f *testing.F) {
	f.Add(0, 0, 1)
	f.Add(16, 0, 1)
	f.Add(16, 4, 32)
	f.Fuzz(func(t *testing.T, winSize, retSize, bufSize int) {
		if winSize < 0 || retSize < 0 || bufSize <= 0 {
			t.Skip()
		}
		if !(retSize < bufSize) {
			t.Skip()
		}
		cfg := ParserConfig{
			WindowSize:    opt.Val(winSize),
			RetentionSize: opt.Val(retSize),
			BufferSize:    bufSize,
			MinMatchLen:   2,
			MaxMatchLen:   273,
		}
		p, err := NewParser(cfg)
		if err != nil {
			t.Fatalf("NewParser: %v", err)
		}
		decCfg := DecoderConfig{
			WindowSize: opt.Val(winSize),
		}
		d, err := NewDecoder(decCfg)
		if err != nil {
			t.Fatalf("NewDecoder: %v", err)
		}

		f, err := os.Open("testdata/enwik7")
		if err != nil {
			t.Fatalf("os.Open: %v", err)
		}
		defer f.Close()
		h := sha256.New()
		r := io.TeeReader(io.LimitReader(f, 10000), h)
		w := sha256.New()

		t.Logf("parser config: %+v", p.Config())
		t.Logf("decoder config: %+v", d.DecoderConfig)
		blockSize := min(128<<10, d.BufferSize-d.WindowSize.V)
		t.Logf("blockSize: %d", blockSize)
		var blk Block
		moreData := true
		for moreData {
			n, err := p.ReadFrom(r)
			if err != nil && err != ErrFullBuffer {
				t.Fatalf("p.ReadFrom: %v", err)
			}
			moreData = err == ErrFullBuffer
			t.Logf("ReadFrom: read %d bytes; moreData=%v",
				n, moreData)

			for {
				k, err := p.Parse(&blk, blockSize, 0)
				if err != nil {
					if err != ErrEndOfBuffer {
						t.Fatalf("p.Parse: %v", err)
					}
					if k == 0 {
						break
					}
				}
				if k > blockSize {
					t.Fatalf(
						"p.Parse: parsed %d bytes; want at most %d",
						k, blockSize)
				}
				t.Logf("p.Parse: parsed %d bytes", k)
				t.Logf("block: %d literals, %d sequences, len %d",
					len(blk.Literals), len(blk.Sequences),
					blk.Len())

				_, err = d.WriteBlock(&blk)
				if err != nil {
					t.Fatalf("d.WriteBlock: %v", err)
				}
				_, cerr := io.Copy(w, d)
				if cerr != nil {
					t.Fatalf("io.Copy: %v", err)
				}
			}
		}

		h1 := h.Sum(nil)
		h2 := w.Sum(nil)

		if !bytes.Equal(h1, h2) {
			t.Fatalf(
				"decoded data differs from original")
		}
	})
}
