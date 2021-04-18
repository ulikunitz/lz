package lz

import (
	"bytes"
	"crypto/sha256"
	"io"
	"math/bits"
	"os"
	"strings"
	"testing"
)

func newTestHashSequencer(tb testing.TB, cfg HSConfig) *HashSequencer {
	hs, err := NewHashSequencer(cfg)
	if err != nil {
		tb.Fatalf("NewHashSequencer(%+v) error %s", cfg, err)
	}
	return hs
}

func newTestBHS(tb testing.TB, cfg HSConfig) *BackwardHashSequencer {
	bhs, err := NewBackwardHashSequencer(cfg)
	if err != nil {
		tb.Fatalf("NewHashSequencer(%+v) error %s", cfg, err)
	}
	return bhs
}

func newTestDHS(tb testing.TB, cfg DHSConfig) *DoubleHashSequencer {
	dhs, err := NewDoubleHashSequencer(cfg)
	if err != nil {
		tb.Fatalf("NewDoubleHashSequencer(%+v) error %s", cfg, err)
	}
	return dhs
}

func TestReset(t *testing.T) {
	const (
		str        = "The quick brown fox jumps over the lazy dogdog."
		windowSize = 20
	)
	hs := newTestHashSequencer(t, HSConfig{
		InputLen:   3,
		WindowSize: windowSize,
		ShrinkSize: windowSize / 4,
		MaxSize:    windowSize,
	})

	r := Wrap(strings.NewReader(str), hs)

	for i := 1; i < 2; i++ {
		var sb strings.Builder
		var d Decoder
		d.Init(&sb, DConfig{WindowSize: windowSize})

		r.Reset(strings.NewReader(str))

		var blk Block
		for {
			_, err := r.Sequence(&blk, 0)
			if err != nil {
				if err == io.EOF {
					break
				}
				t.Fatalf("r.Sequence(&blk, 0) error %s", err)
			}

			d.WriteBlock(blk)
		}

		if err := d.Flush(); err != nil {
			t.Fatalf("d,Flush error %s", err)
		}

		g := sb.String()
		if g != str {
			t.Fatalf("%d: got %q; want %q", i, g, str)
		}
	}
}

// blockCost computes the cost of the block in bytes
func blockCost(blk *Block) int64 {
	c := int64(0)
	for _, seq := range blk.Sequences {
		c += int64(bits.Len32(seq.LitLen))
		c += int64(bits.Len32(seq.MatchLen))
		c += int64(bits.Len32(seq.Offset))
	}
	c += 8 * int64(len(blk.Literals))
	return c
}

func TestSequencers(t *testing.T) {
	const enwik7 = "testdata/enwik7"
	tests := []struct {
		name string
		ws   WriteSequencer
	}{
		{
			name: "HashSequencer-3",
			ws: newTestHashSequencer(t, HSConfig{
				InputLen:   3,
				WindowSize: 8 << 20,
				ShrinkSize: 32 << 10,
				MaxSize:    8 << 20,
			}),
		},
		{
			name: "BackwardHashSequencer-3",
			ws: newTestBHS(t, HSConfig{
				InputLen:   3,
				WindowSize: 8 << 20,
				ShrinkSize: 32 << 10,
				MaxSize:    8 << 20,
			}),
		},
		{
			name: "DoubleHashSequencer-3,8",
			ws: newTestDHS(t, DHSConfig{
				InputLen1:  3,
				InputLen2:  8,
				WindowSize: 8 << 20,
				ShrinkSize: 32 << 10,
				MaxSize:    8 << 20,
			}),
		},
	}
	data, err := os.ReadFile(enwik7)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error %s", enwik7, err)
	}
	hd := sha256.New()
	hd.Write(data)
	sumData := hd.Sum(nil)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := sha256.New()
			winSize := tc.ws.WindowSize()
			d, err := NewDecoder(h, DConfig{
				WindowSize: winSize,
				MaxSize:    2 * winSize})
			if err != nil {
				t.Fatalf("NewDecoder error %s", err)
			}

			s := Wrap(bytes.NewReader(data), tc.ws)

			var blk Block
			for {
				_, err := s.Sequence(&blk, 0)
				if err != nil {
					if err == io.EOF {
						break
					}
					t.Fatalf("s.Sequencer error %s",
						err)
				}

				_, _, _, err = d.WriteBlock(blk)
				if err != nil {
					t.Fatalf("d.WriteBlock error %s",
						err)
				}
			}

			if err = d.Flush(); err != nil {
				t.Fatalf("d.Flush() error %s", err)
			}

			sum := h.Sum(nil)
			if !bytes.Equal(sum, sumData) {
				t.Fatalf("hash Is %x; want %x", sum, sumData)
			}
		})

	}

}

func TestSequencersSimple(t *testing.T) {
	const str = "=====FoobardeFoobarde======"
	tests := []struct {
		name string
		ws   WriteSequencer
	}{
		{
			name: "HashSequencer-3",
			ws: newTestHashSequencer(t, HSConfig{
				InputLen:   3,
				WindowSize: 8 << 20,
				ShrinkSize: 32 << 10,
				MaxSize:    8 << 20,
			}),
		},
		{
			name: "BackwardHashSequencer-3",
			ws: newTestBHS(t, HSConfig{
				InputLen:   3,
				WindowSize: 8 << 20,
				ShrinkSize: 32 << 10,
				MaxSize:    8 << 20,
			}),
		},
		{
			name: "DoubleHashSequencer-3,6",
			ws: newTestDHS(t, DHSConfig{
				InputLen1:  3,
				InputLen2:  6,
				WindowSize: 8 << 20,
				ShrinkSize: 32 << 10,
				MaxSize:    8 << 20,
			}),
		},
	}
	data := []byte(str)
	hd := sha256.New()
	hd.Write(data)
	sumData := hd.Sum(nil)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := sha256.New()
			winSize := tc.ws.WindowSize()
			d, err := NewDecoder(h, DConfig{
				WindowSize: winSize,
				MaxSize:    2 * winSize})
			if err != nil {
				t.Fatalf("NewDecoder error %s", err)
			}

			s := Wrap(bytes.NewReader(data), tc.ws)

			var blk Block
			for {
				_, err := s.Sequence(&blk, 0)
				if err != nil {
					if err == io.EOF {
						break
					}
					t.Fatalf("s.Sequencer error %s",
						err)
				}

				_, _, _, err = d.WriteBlock(blk)
				if err != nil {
					t.Fatalf("d.WriteBlock error %s",
						err)
				}
			}

			if err = d.Flush(); err != nil {
				t.Fatalf("d.Flush() error %s", err)
			}

			sum := h.Sum(nil)
			if !bytes.Equal(sum, sumData) {
				t.Fatalf("hash Is %x; want %x", sum, sumData)
			}
		})

	}

}

func BenchmarkSequencers(b *testing.B) {
	const enwik7 = "testdata/enwik7"
	benchmarks := []struct {
		name string
		ws   WriteSequencer
	}{
		{"HashSequencer-3", newTestHashSequencer(b, HSConfig{
			InputLen:   3,
			HashBits:   15,
			WindowSize: 8 << 20,
			ShrinkSize: 32 << 10,
			MaxSize:    8 << 20,
		})},
		{"HashSequencer-4", newTestHashSequencer(b, HSConfig{
			InputLen:   4,
			HashBits:   15,
			WindowSize: 8 << 20,
			ShrinkSize: 32 << 10,
			MaxSize:    8 << 20,
		})},
		{"HashSequencer-5", newTestHashSequencer(b, HSConfig{
			InputLen:   5,
			HashBits:   15,
			WindowSize: 8 << 20,
			ShrinkSize: 32 << 10,
			MaxSize:    8 << 20,
		})},
		{"HashSequencer-8", newTestHashSequencer(b, HSConfig{
			InputLen:   8,
			WindowSize: 8 << 20,
			ShrinkSize: 32 << 10,
			MaxSize:    8 << 20,
		})},
		{"BackwardHashSequencer-3", newTestBHS(b, HSConfig{
			InputLen:   3,
			HashBits:   15,
			WindowSize: 8 << 20,
			ShrinkSize: 32 << 10,
			MaxSize:    8 << 20,
		})},
		{"BackwardHashSequencer-4", newTestBHS(b, HSConfig{
			InputLen:   4,
			HashBits:   15,
			WindowSize: 8 << 20,
			ShrinkSize: 32 << 10,
			MaxSize:    8 << 20,
		})},
		{"BackwardHashSequencer-5", newTestBHS(b, HSConfig{
			InputLen:   5,
			HashBits:   15,
			WindowSize: 8 << 20,
			ShrinkSize: 32 << 10,
			MaxSize:    8 << 20,
		})},
		{"BackwardHashSequencer-8", newTestBHS(b, HSConfig{
			InputLen:   8,
			HashBits:   15,
			WindowSize: 8 << 20,
			ShrinkSize: 32 << 10,
			MaxSize:    8 << 20,
		})},
		{"DoubleHashSequencer-3,6", newTestDHS(b, DHSConfig{
			InputLen1:  3,
			InputLen2:  6,
			HashBits1:  15,
			HashBits2:  18,
			WindowSize: 8 << 20,
			ShrinkSize: 32 << 10,
			MaxSize:    8 << 20,
		})},
		{"DoubleHashSequencer-4,6", newTestDHS(b, DHSConfig{
			InputLen1:  4,
			InputLen2:  6,
			HashBits1:  15,
			HashBits2:  18,
			WindowSize: 8 << 20,
			ShrinkSize: 32 << 10,
			MaxSize:    8 << 20,
		})},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			data, err := os.ReadFile(enwik7)
			if err != nil {
				b.Fatalf("io.ReadFile(%q) error %s", enwik7,
					err)
			}
			r := Wrap(bytes.NewReader(data), bm.ws)
			b.SetBytes(int64(len(data)))
			var cost int64
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				var blk Block
			loop:
				for {
					_, err := r.Sequence(&blk, 0)
					b.StopTimer()
					cost += blockCost(&blk)
					b.StartTimer()
					switch err {
					case nil:
						continue loop
					case io.EOF:
						break loop
					default:
						b.Fatalf("r.Sequence(&blk) error %s", err)
					}
				}
				b.StopTimer()
				r.Reset(bytes.NewReader(data))
				b.StartTimer()
			}
			b.StopTimer()
			compressedBytes := (cost + 7) / 8
			uncompressedBytes := int64(b.N) * int64(len(data))
			b.ReportMetric(
				100*float64(compressedBytes)/
					float64(uncompressedBytes),
				"%_compression_ratio")
		})
	}
}

func BenchmarkDecoders(b *testing.B) {
	const enwik7 = "testdata/enwik7"
	benchmarks := []struct {
		name    string
		winSize int
		maxSize int
		ring    bool
	}{
		{name: "Decoder", winSize: 1024 * 1024},
		{name: "RingDecoder", winSize: 1024 * 1024, ring: true},
	}
	data, err := os.ReadFile(enwik7)
	if err != nil {
		b.Fatalf("os.ReadFile(%q) error %s", enwik7, err)
	}
	hd := sha256.New()
	hd.Write(data)
	sumData := hd.Sum(nil)
	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			var blocks []Block
			hs, err := NewHashSequencer(HSConfig{
				InputLen:   3,
				WindowSize: bm.winSize,
				MaxSize:    bm.maxSize,
				ShrinkSize: bm.winSize,
			})
			if err != nil {
				b.Fatalf("NewHashSequencer error %s", err)
			}
			s := Wrap(bytes.NewReader(data), hs)
			for {
				var blk Block
				_, err = s.Sequence(&blk, 0)
				if err != nil {
					if err == io.EOF {
						break
					}
					b.Fatalf("s.Sequence error %s", err)
				}
				blocks = append(blocks, blk)
			}
			b.SetBytes(int64(len(data)))

			var d interface {
				WriteBlock(blk Block) (k, l int, n int64,
					err error)
				Flush() error
				Reset(w io.Writer)
			}
			hw := sha256.New()
			if bm.ring {
				d, err = NewRingDecoder(hw, bm.winSize)
				if err != nil {
					b.Fatalf("NewRingDecoder error %s", err)
				}
			} else {

				d, err = NewDecoder(hw, DConfig{
					WindowSize: bm.winSize,
					MaxSize:    bm.maxSize,
				})
				if err != nil {
					b.Fatalf("NewDecoder error %s", err)
				}
			}
			b.ResetTimer()
			b.StopTimer()
			for i := 0; i < b.N; i++ {
				hw.Reset()
				b.StartTimer()
				d.Reset(hw)
				for _, blk := range blocks {
					_, _, _, err := d.WriteBlock(blk)
					if err != nil {
						b.Fatalf("d.WriteBlock"+
							" error %s",
							err)
					}
				}
				if err = d.Flush(); err != nil {
					b.Fatalf("d.Flush() error %s", err)
				}
				b.StopTimer()
				sum := hw.Sum(nil)
				if !bytes.Equal(sum, sumData) {
					b.Fatalf("got hash %x; want %x", sum,
						sumData)
				}
			}
		})
	}
}

func TestBHSSimple(t *testing.T) {
	const str = "=====foofoobarfoobar bartender===="

	var s BackwardHashSequencer
	if err := s.Init(HSConfig{
		WindowSize: 1024,
		ShrinkSize: 1024,
		BlockSize:  512,
		MaxSize:    2 * 1024,
		InputLen:   3,
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
	if err := d.Init(&buf, DConfig{WindowSize: 1024}); err != nil {
		t.Fatalf("dw.Init(%d) error %s", 1024, err)
	}
	k, l, m, err := d.WriteBlock(blk)
	if err != nil {
		t.Fatalf("dw.WriteBlock(blk) error %s", err)
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
		t.Fatalf("dw.WriteBlock(blk) returned %d; want %d bytes",
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
