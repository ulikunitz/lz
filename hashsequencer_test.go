package lz

import (
	"bytes"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

func TestHashSequencerSimple(t *testing.T) {
	const str = "=====foofoobarfoobar bartender===="

	var s HashSequencer
	if err := s.Init(HashSequencerConfig{
		WindowSize:  1024,
		ShrinkSize:  1024,
		BlockSize:   512,
		MaxSize:     2 * 1024,
		InputLen:    3,
		MinMatchLen: 2,
	}); err != nil {
		t.Fatalf("s.Init error %s", err)
	}
	n, err := s.Write([]byte(str))
	if err != nil {
		t.Fatalf("s.Write(%q) error %s", str, err)
	}
	if n != len(str) {
		t.Fatalf("s.Write(%q) returned %d; want %d", str, n, len(str))
	}

	var blk Block
	n, err = s.Sequence(&blk, 0)
	if err != nil {
		t.Fatalf("s.Sequence error %s", err)
	}
	if n != len(str) {
		t.Fatalf("s.Sequence returned %d; want %d", n, len(str))
	}
	t.Logf("sequences: %+v", blk.Sequences)
	t.Logf("literals: %q", blk.Literals)
	if len(blk.Sequences) == 0 {
		t.Errorf("len(blk.Sequences)=%d; want value > 0",
			len(blk.Sequences))
	}
	if len(blk.Literals) >= len(str) {
		t.Errorf("len(blk.Literals)=%d; should < %d",
			len(blk.Literals), len(str))
	}

	var buf bytes.Buffer
	var d Decoder
	if err := d.Init(&buf, 1024); err != nil {
		t.Fatalf("dw.Init(%d) error %s", 1024, err)
	}
	k, l, m, err := d.WriteBlock(&blk)
	if err != nil {
		t.Fatalf("dw.WriteBlock(&blk) error %s", err)
	}
	if k != len(blk.Sequences) {
		t.Fatalf("dw.WriteBlock returned k=%d; want %d sequences",
			k, len(blk.Sequences))
	}
	if l != len(blk.Literals) {
		t.Fatalf("dw.WriteBlock returned l=%d; want %d literals",
			l, len(blk.Literals))
	}
	if m != int64(len(str)) {
		t.Fatalf("dw.WriteBlock(&blk) returned %d; want %d bytes",
			m, len(str))
	}
	if err = d.Flush(); err != nil {
		t.Fatalf("d.Flush() error %s", err)
	}

	g := buf.String()
	if g != str {
		t.Fatalf("uncompressed string %q; want %q", g, str)
	}
	t.Logf("g: %q", g)
}

func TestWrapHashSequencer(t *testing.T) {
	const (
		windowSize = 1024
		blockSize  = 512
		str        = "=====foofoobarfoobar bartender===="
	)

	ws, err := NewHashSequencer(HashSequencerConfig{
		WindowSize:  windowSize,
		ShrinkSize:  windowSize,
		BlockSize:   blockSize,
		MaxSize:     2 * windowSize,
		InputLen:    3,
		MinMatchLen: 2,
	})
	if err != nil {
		t.Fatalf("NewHashSequencer error %s", err)
	}
	s := Wrap(strings.NewReader(str), ws)

	var builder strings.Builder
	var decoder Decoder
	decoder.Init(&builder, windowSize)

	var blk Block
	for {
		if _, err := s.Sequence(&blk, 0); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("s.Sequence error %s", err)
		}
		if _, _, _, err := decoder.WriteBlock(&blk); err != nil {
			t.Fatalf("decoder.WriteBlock error %s", err)
		}
	}
	if err := decoder.Flush(); err != nil {
		t.Fatalf("decoder.Flush error %s", err)
	}

	g := builder.String()
	if g != str {
		t.Fatalf("got string %q; want %q", g, str)
	}
}

func TestHashSequencerEnwik7(t *testing.T) {
	const (
		enwik7     = "testdata/enwik7"
		blockSize  = 128 * 1024
		windowSize = 2*blockSize + 123
	)
	f, err := os.Open(enwik7)
	if err != nil {
		t.Fatalf("os.Open(%q) error %s", enwik7, err)
	}
	defer func() {
		if err = f.Close(); err != nil {
			t.Fatalf("f.Close() error %s", err)
		}
	}()
	h1 := sha256.New()
	r := io.TeeReader(f, h1)

	cfg := HashSequencerConfig{
		BlockSize:   blockSize,
		WindowSize:  windowSize,
		ShrinkSize:  windowSize / 4,
		MaxSize:     2 * windowSize,
		InputLen:    3,
		MinMatchLen: 2,
	}
	ws, err := NewHashSequencer(cfg)
	if err != nil {
		t.Fatalf("NewHashSequencer(%+v) error %s", cfg, err)
	}
	s := Wrap(r, ws)

	h2 := sha256.New()
	var decoder Decoder
	if err = decoder.Init(h2, windowSize); err != nil {
		t.Fatalf("decoder.Init() error %s", err)
	}

	var blk Block
	for {
		_, err = s.Sequence(&blk, 0)
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("s.Sequence error %s", err)
		}
		if len(blk.Sequences) == 0 {
			t.Fatalf("s.Sequence doesn't compress")
		}
		if _, _, _, err := decoder.WriteBlock(&blk); err != nil {
			t.Fatalf("decoder.WriteBlock error %s", err)
		}
	}
	if err := decoder.Flush(); err != nil {
		t.Fatalf("decoder.Flush error %s", err)
	}

	sum1 := h1.Sum(nil)
	sum2 := h2.Sum(nil)

	if !bytes.Equal(sum1, sum2) {
		t.Fatalf("decoded hash sum: %x; want %x", sum2, sum1)
	}
}

// TODO: add benchmarks for HashSequencer and for Encoder.
// Implement the sequencer in a ways that it can be used with multiple
// configurations and setups

type loopReader struct {
	r io.ReadSeeker
}

func (ir *loopReader) Read(p []byte) (n int, err error) {
	n, err = ir.r.Read(p)
	if err != io.EOF {
		return n, err
	}
	if _, err = ir.r.Seek(0, io.SeekStart); err != nil {
		return n, err
	}
	if n == len(p) {
		return n, nil
	}
	var k int
	k, err = ir.r.Read(p[n:])
	n += k
	return n, err

}

func newLoopReader(r io.ReadSeeker) *loopReader {
	return &loopReader{r}
}

func TestLoopReader(t *testing.T) {
	const str = "The brown fox jumps ove the lazy dog."

	const size = 128
	r := io.LimitReader(newLoopReader(strings.NewReader(str)), size)

	var sb strings.Builder
	n, err := io.Copy(&sb, r)
	if err != nil {
		t.Fatalf("io.Copy(&sb, r) error %s", err)
	}
	if n != size {
		t.Fatalf("io.Copy(&sb. r) returne %d; want %d", n, size)
	}
	t.Logf("%q", sb.String())
}

// TODO: test behavoir for large limits

var largeFlag = flag.Bool("large", false, "test large parameters")

func TestLargeParameters(t *testing.T) {
	if !*largeFlag {
		t.Skipf("use -large flag to execute test")
	}
	if testing.Short() {
		t.Skipf("test is slow")
	}
	const enwik7 = "testdata/enwik7"

	var tests = []struct {
		filename string
		size     int64
		cfg      HashSequencerConfig
	}{
		{enwik7, 9 << 30, HashSequencerConfig{
			InputLen:    3,
			MinMatchLen: 3,
			BlockSize:   128 * 1024,
			WindowSize:  8 << 20,
			ShrinkSize:  1 << 20,
			MaxSize:     maxUint32,
		}},
	}

	for i, tc := range tests {
		tc := tc
		t.Run(fmt.Sprintf("%d", i+1), func(t *testing.T) {
			f, err := os.Open(tc.filename)
			if err != nil {
				t.Fatalf("os.Open(%q) error %s", tc.filename,
					err)
			}
			defer func() {
				if err := f.Close(); err != nil {
					t.Fatalf("f.Close() error %s", err)
				}
			}()
			r := io.LimitReader(newLoopReader(f), tc.size)
			ws, err := NewHashSequencer(tc.cfg)
			if err != nil {
				t.Fatalf("NewHashSequencer(%+v) error %s",
					tc.cfg, err)
			}
			h1, h2 := sha256.New(), sha256.New()
			s := Wrap(io.TeeReader(r, h1), ws)

			var d Decoder
			d.Init(h2, tc.cfg.WindowSize)

			var blk Block
			var n int64
			for {
				k, err := s.Sequence(&blk, 0)
				n += int64(k)
				if err != nil {
					if err == io.EOF {
						break
					}
					t.Fatalf("s.Sequence(&blk, 0) error %s",
						err)
				}

				if len(blk.Sequences) == 0 {
					t.Fatalf("no compression")
				}

				_, _, _, err = d.WriteBlock(&blk)
				if err != nil {
					t.Fatalf("d.WriteBlock(&blk) error %s",
						err)
				}

			}

			if err = d.Flush(); err != nil {
				t.Fatalf("d.Flush() error %s", err)
			}

			s1, s2 := h1.Sum(nil), h2.Sum(nil)
			if !bytes.Equal(s1, s2) {
				t.Fatalf("decompressed hash %x; want %x",
					s1, s2)
			}
		})
	}
}
