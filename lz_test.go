// SPDX-FileCopyrightText: © 2021 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

package lz

import (
	"bytes"
	"crypto/sha256"
	"io"
	"os"
	"testing"
)

/*
func FuzzBHP(f *testing.F) {
	f.Add(3, 5, []byte("=====foofoobarfoobar bartender===="))
	f.Fuzz(func(t *testing.T, inputLen int, hashBits int, p []byte) {
		cfg := &BHPConfig{
			WindowSize: 1024,
			BlockSize:  512,
			InputLen:   inputLen,
			HashBits:   hashBits,
		}
		testParser(t, cfg, p)
	})
}

func FuzzDHP(f *testing.F) {
	f.Add(3, 5, 4, 6, []byte("=====foofoobarfoobar bartender===="))
	f.Fuzz(func(t *testing.T,
		inputLen1, hashBits1 int,
		inputLen2, hashBits2 int,
		p []byte) {

		cfg := &DHPConfig{
			WindowSize: 1024,
			BlockSize:  512,
			InputLen1:  inputLen1,
			HashBits1:  hashBits1,
			InputLen2:  inputLen2,
			HashBits2:  hashBits2,
		}
		testParser(t, cfg, p)
	})
}

func FuzzBDHP(f *testing.F) {
	f.Add(3, 5, 4, 6, []byte("=====foofoobarfoobar bartender===="))
	f.Fuzz(func(t *testing.T,
		inputLen1, hashBits1 int,
		inputLen2, hashBits2 int,
		p []byte) {

		cfg := &BDHPConfig{
			WindowSize: 1024,
			BlockSize:  512,
			InputLen1:  inputLen1,
			InputLen2:  inputLen2,
			HashBits1:  hashBits1,
			HashBits2:  hashBits2,
		}
		testParser(t, cfg, p)
	})
}

func FuzzBUP(f *testing.F) {
	f.Add(3, 5, 8, []byte("=====foofoobarfoobar bartender===="))
	f.Fuzz(func(t *testing.T,
		inputLen, hashBits, bucketSize int,
		p []byte) {

		cfg := &BUPConfig{
			WindowSize: 1024,
			BlockSize:  512,
			InputLen:   inputLen,
			HashBits:   hashBits,
			BucketSize: bucketSize,
		}
		cfg.SetDefaults()
		// We need to limit the memory consumption for Fuzzing.
		if cfg.HashBits > 21 {
			t.Skip()
		}
		testParser(t, cfg, p)
	})
}

func FuzzGSAP(f *testing.F) {
	f.Add([]byte("=====foofoobarfoobar bartender===="))
	f.Fuzz(func(t *testing.T, p []byte) {
		cfg := &GSAPConfig{
			WindowSize: 1024,
			BlockSize:  512,
		}
		testParser(t, cfg, p)
	})
}

func FuzzOSAP(f *testing.F) {
	f.Add([]byte("abbababb"))
	f.Add([]byte("=====foofoobarfoobar bartender===="))
	f.Fuzz(func(t *testing.T, p []byte) {
		cfg := &OSAPConfig{
			BufferSize: 1024,
			WindowSize: 1024,
			BlockSize:  512,
		}
		testParser(t, cfg, p)
	})
}

func newTestParser(tb testing.TB, cfg ParserConfig) Parser {
	s, err := cfg.NewParser()
	if err != nil {
		tb.Fatalf("%+v.NewParser() error %s",
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

func BenchmarkParsers(b *testing.B) {
	const enwik7 = "testdata/enwik7"
	benchmarks := []struct {
		name string
		cfg  ParserConfig
	}{
		{"HashParser-3", &HPConfig{
			WindowSize: 8 << 20,
			InputLen:   3,
			HashBits:   15,
		}},
		{"HashParser-4", &HPConfig{
			InputLen:   4,
			HashBits:   15,
			WindowSize: 8 << 20,
		}},
		{"HashParser-5", &HPConfig{
			InputLen:   5,
			HashBits:   15,
			WindowSize: 8 << 20,
		}},
		{"HashParser-8", &HPConfig{
			InputLen:   8,
			HashBits:   15,
			WindowSize: 8 << 20,
		}},
		{"BackwardHashParser-3", &BHPConfig{
			InputLen:   3,
			HashBits:   15,
			WindowSize: 8 << 20,
		}},
		{"BackwardHashParser-4", &BHPConfig{
			InputLen:   4,
			HashBits:   15,
			WindowSize: 8 << 20,
		}},
		{"BackwardHashParser-5", &BHPConfig{
			InputLen:   5,
			HashBits:   15,
			WindowSize: 8 << 20,
		}},
		{"BackwardHashParser-8", &BHPConfig{
			InputLen:   8,
			HashBits:   15,
			WindowSize: 8 << 20,
		}},
		{"DoubleHashParser-3,6", &DHPConfig{
			InputLen1:  2,
			HashBits1:  15,
			InputLen2:  6,
			HashBits2:  18,
			WindowSize: 8 << 20,
		}},
		{"DoubleHashParser-4,6", &DHPConfig{
			InputLen1:  4,
			HashBits1:  15,
			InputLen2:  6,
			HashBits2:  18,
			WindowSize: 8 << 20,
		}},
		{"BDHParser-3,6", &BDHPConfig{
			InputLen1:  3,
			HashBits1:  15,
			InputLen2:  6,
			HashBits2:  18,
			WindowSize: 8 << 20,
		}},
		{"BDHParser-4,6", &BDHPConfig{
			InputLen1:  4,
			HashBits1:  15,
			InputLen2:  6,
			HashBits2:  18,
			WindowSize: 8 << 20,
		}},
		{"GSAParser", &GSAPConfig{
			WindowSize: 8 << 20,
		}},
		{"BUParser-3-12", &BUPConfig{
			InputLen:   3,
			HashBits:   18,
			BucketSize: 12,
			WindowSize: 8 << 20,
		}},
		{"BUParser-3-100", &BUPConfig{
			InputLen:   3,
			HashBits:   18,
			BucketSize: 100,
			WindowSize: 8 << 20,
		}},
		{"OSAParser", &OSAPConfig{
			MinMatchLen: 2,
			MaxMatchLen: 273,
			Cost:        "XZCost",
			WindowSize:  512 << 10,
			BufferSize:  512 << 10,
		}},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			ws := newTestParser(b, bm.cfg)
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
					_, err := r.Parse(&blk, 0)
					b.StopTimer()
					cost += blockCost(&blk)
					b.StartTimer()
					switch err {
					case nil:
						continue loop
					case io.EOF:
						break loop
					default:
						b.Fatalf("r.Parse(&blk) error %s", err)
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
*/

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
			hs, err := NewHashParser(
				HashConfig{InputLen: 3},
				BufConfig{WindowSize: bm.winSize},
			)
			if err != nil {
				b.Fatalf("NewHashParser error %s", err)
			}
			s := Wrap(bytes.NewReader(data), hs)
			for {
				var blk Block
				_, err = s.Parse(&blk, 0)
				if err != nil {
					if err == io.EOF {
						break
					}
					b.Fatalf("s.Parse error %s", err)
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

			d, err = NewDecoder(hw, DecoderConfig{
				WindowSize: bm.winSize,
				BufferSize: bm.maxSize,
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
