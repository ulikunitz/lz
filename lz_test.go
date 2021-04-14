package lz

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func newTestHashSequencer(tb testing.TB, cfg HashSequencerConfig) *HashSequencer {
	hs, err := NewHashSequencer(cfg)
	if err != nil {
		tb.Fatalf("NewHashSequencer(%+v) error %s", cfg, err)
	}
	return hs
}

func TestReset(t *testing.T) {
	const (
		str        = "The quick brown fox jumps over the lazy dogdog."
		windowSize = 20
	)
	hs := newTestHashSequencer(t, HashSequencerConfig{
		InputLen:    3,
		MinMatchLen: 3,
		WindowSize:  windowSize,
		ShrinkSize:  windowSize / 4,
		MaxSize:     windowSize,
	})

	r := Wrap(strings.NewReader(str), hs)

	for i := 1; i < 2; i++ {
		var sb strings.Builder
		var d Decoder
		d.Init(&sb, windowSize)

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

			d.WriteBlock(&blk)
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

func BenchmarkSequencers(b *testing.B) {
	const enwik7 = "testdata/enwik7"
	benchmarks := []struct {
		name string
		ws   WriteSequencer
	}{
		{"HashSequencer", newTestHashSequencer(b, HashSequencerConfig{
			InputLen:    4,
			MinMatchLen: 3,
			WindowSize:  8 << 20,
			ShrinkSize:  32 << 10,
			MaxSize:     8 << 20,
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
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				var blk Block
			loop:
				for {
					_, err := r.Sequence(&blk, 0)
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
		})
	}
}
