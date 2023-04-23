package lz

import (
	"bytes"
	"crypto/sha256"
	"io"
	"math/bits"
	"os"
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
			HConfig{
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
				H1: HConfig{inputLen1, hashBits1},
				H2: HConfig{inputLen2, hashBits2},
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
				H1: HConfig{inputLen1, hashBits1},
				H2: HConfig{inputLen2, hashBits2},
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

func FuzzOSAS(f *testing.F) {
	f.Add([]byte("abbababb"))
	f.Add([]byte("=====foofoobarfoobar bartender===="))
	f.Fuzz(func(t *testing.T, p []byte) {
		cfg := &OSASConfig{
			BufConfig: BufConfig{
				BufferSize: 1024,
				WindowSize: 1024,
				BlockSize:  512,
			},
		}
		testSequencer(t, cfg, p)
	})
}

func newTestSequencer(tb testing.TB, cfg SeqConfig) Sequencer {
	s, err := cfg.NewSequencer()
	if err != nil {
		tb.Fatalf("%+v.NewSequencer() error %s",
			cfg, err)
	}
	return s
}

// blockCost computes the cost of the block in bits.
func blockCost(blk *Block) int64 {
	c := int64(0)
	for _, seq := range blk.Sequences {
		l := int64(seq.MatchLen)
		l -= 2
		switch {
		case l < 8:
			c += 4
		case l < 16:
			c += 5
		default:
			c += 10
		}
		d := int64(seq.Offset) - 1
		if d < 4 {
			c += 4
		} else {
			c += 2 + int64(bits.Len64(uint64(d)))
		}
	}
	c += 9 * int64(len(blk.Literals))
	return c
}

func BenchmarkSequencers(b *testing.B) {
	const enwik7 = "testdata/enwik7"
	benchmarks := []struct {
		name string
		cfg  SeqConfig
	}{
		{"HashSequencer-3", &HSConfig{
			BufConfig: BufConfig{
				WindowSize: 8 << 20,
			},
			HConfig: HConfig{
				InputLen: 3,
				HashBits: 15,
			},
		}},
		{"HashSequencer-4", &HSConfig{
			HConfig: HConfig{
				InputLen: 4,
				HashBits: 15,
			},
			BufConfig: BufConfig{
				WindowSize: 8 << 20,
			},
		}},
		{"HashSequencer-5", &HSConfig{
			HConfig: HConfig{
				InputLen: 5,
				HashBits: 15,
			},
			BufConfig: BufConfig{
				WindowSize: 8 << 20,
			},
		}},
		{"HashSequencer-8", &HSConfig{
			HConfig: HConfig{
				InputLen: 8,
				HashBits: 15,
			},
			BufConfig: BufConfig{
				WindowSize: 8 << 20,
			},
		}},
		{"BackwardHashSequencer-3", &BHSConfig{
			HConfig: HConfig{
				InputLen: 3,
				HashBits: 15,
			},
			BufConfig: BufConfig{
				WindowSize: 8 << 20,
			},
		}},
		{"BackwardHashSequencer-4", &BHSConfig{
			HConfig: HConfig{
				InputLen: 4,
				HashBits: 15,
			},
			BufConfig: BufConfig{
				WindowSize: 8 << 20,
			},
		}},
		{"BackwardHashSequencer-5", &BHSConfig{
			HConfig: HConfig{
				InputLen: 5,
				HashBits: 15,
			},
			BufConfig: BufConfig{
				WindowSize: 8 << 20,
			},
		}},
		{"BackwardHashSequencer-8", &HSConfig{
			HConfig: HConfig{
				InputLen: 8,
				HashBits: 15,
			},
			BufConfig: BufConfig{
				WindowSize: 8 << 20,
			},
		}},
		{"DoubleHashSequencer-3,6", &DHSConfig{
			DHConfig: DHConfig{
				H1: HConfig{
					InputLen: 3, HashBits: 15,
				},
				H2: HConfig{
					InputLen: 6, HashBits: 18,
				},
			},
			BufConfig: BufConfig{
				WindowSize: 8 << 20,
			},
		}},
		{"DoubleHashSequencer-4,6", &DHSConfig{
			DHConfig: DHConfig{
				H1: HConfig{
					InputLen: 4,
					HashBits: 15,
				},
				H2: HConfig{
					InputLen: 6,
					HashBits: 18,
				},
			},
			BufConfig: BufConfig{
				WindowSize: 8 << 20,
			},
		}},
		{"BDHSequencer-3,6", &BDHSConfig{
			DHConfig: DHConfig{
				H1: HConfig{
					InputLen: 3,
					HashBits: 15,
				},
				H2: HConfig{
					InputLen: 6,
					HashBits: 18,
				},
			},
			BufConfig: BufConfig{
				WindowSize: 8 << 20,
			},
		}},
		{"BDHSequencer-4,6", &BDHSConfig{
			DHConfig: DHConfig{
				H1: HConfig{
					InputLen: 4,
					HashBits: 15,
				},
				H2: HConfig{
					InputLen: 6,
					HashBits: 18,
				},
			},
			BufConfig: BufConfig{
				WindowSize: 8 << 20,
			},
		}},
		{"GSASequencer", &GSASConfig{
			BufConfig: BufConfig{
				WindowSize: 8 << 20,
			},
		}},
		{"BUHSequencer-3-12", &BUHSConfig{
			BUHConfig: BUHConfig{
				InputLen:   3,
				HashBits:   18,
				BucketSize: 12,
			},
			BufConfig: BufConfig{
				WindowSize: 8 << 20,
			},
		}},
		{"BUHSequencer-3-100", &BUHSConfig{
			BUHConfig: BUHConfig{
				InputLen:   3,
				HashBits:   18,
				BucketSize: 100,
			},
			BufConfig: BufConfig{
				WindowSize: 8 << 20,
			},
		}},
		{"OSASequencer", &OSASConfig{
			MinMatchLen: 2,
			MaxMatchLen: 273,
			Cost:        XZCost,
			BufConfig: BufConfig{
				WindowSize: 512 << 10,
				BufferSize: 512 << 10,
			},
		}},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			ws := newTestSequencer(b, bm.cfg)
			data, err := os.ReadFile(enwik7)
			if err != nil {
				b.Fatalf("io.ReadFile(%q) error %s", enwik7,
					err)
			}
			r := Wrap(bytes.NewReader(data), ws)
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
	}{
		{name: "Decoder", winSize: 1024 * 1024},
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
			hs, err := HSConfig{
				HConfig: HConfig{InputLen: 3},
				BufConfig: BufConfig{
					WindowSize: bm.winSize,
				},
			}.NewSequencer()

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
				WriteBlock(blk Block) (k, l, n int, err error)
				Flush() error
				Reset(w io.Writer)
			}
			hw := sha256.New()

			d, err = NewDecoder(hw, DConfig{
				WindowSize: bm.winSize,
				MaxSize:    bm.maxSize,
			})
			if err != nil {
				b.Fatalf("NewDecoder error %s", err)
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
