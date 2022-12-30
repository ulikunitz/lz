package lz

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"math/bits"
	"os"
	"strings"
	"testing"
)

func newTestSequencer(tb testing.TB, cfg SeqConfig) Sequencer {
	s, err := cfg.NewSequencer()
	if err != nil {
		tb.Fatalf("%+v.NewSequencer() error %s",
			cfg, err)
	}
	return s
}

func TestReset(t *testing.T) {
	const (
		str        = "The quick brown fox jumps over the lazy dogdog."
		windowSize = 20
		blockSize  = 512
	)

	hs := newTestSequencer(t, &HSConfig{
		InputLen: 3,
		SBConfig: SBConfig{
			WindowSize: windowSize,
			BlockSize:  blockSize,
		},
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
		cfg  SeqConfig
	}{
		{
			name: "HashSequencer-3",
			cfg: &HSConfig{
				InputLen: 3,
				SBConfig: SBConfig{
					WindowSize: 8 << 20,
				},
			},
		},
		{
			name: "BackwardHashSequencer-3",
			cfg: &BHSConfig{
				InputLen: 3,
				SBConfig: SBConfig{
					WindowSize: 8 << 20,
				},
			},
		},
		{
			name: "DoubleHashSequencer-3,8",
			cfg: &DHSConfig{
				InputLen1: 3,
				InputLen2: 8,
				SBConfig: SBConfig{
					WindowSize: 8 << 20,
				},
			},
		},
		{
			name: "BDHSequencer-3,8",
			cfg: &BDHSConfig{
				InputLen1: 3,
				InputLen2: 8,
				SBConfig: SBConfig{
					WindowSize: 8 << 20,
				},
			},
		},
		{
			name: "GSASequencer",
			cfg: &GSASConfig{
				SBConfig: SBConfig{
					WindowSize: 8 << 20,
				},
			},
		},
		{
			name: "BucketHashSequencer",
			cfg: &BUHSConfig{
				SBConfig: SBConfig{
					WindowSize: 8 << 20,
				},
			},
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
			ws := newTestSequencer(t, tc.cfg)
			hOrig := sha256.New()
			h := sha256.New()
			b := ws.Buffer()
			winSize := b.WindowSize
			d, err := NewDecoder(h, DConfig{WindowSize: winSize})
			if err != nil {
				t.Fatalf("NewDecoder error %s", err)
			}

			s := Wrap(bytes.NewReader(data), ws)

			n := 0
			var blk Block
			for {

				k, err := s.Sequence(&blk, 0)
				if err != nil {
					if err == io.EOF {
						break
					}
					t.Fatalf("s.Sequencer error %s",
						err)
				}
				hOrig.Write(data[n : n+k])
				n += k
				sumOrig := hOrig.Sum(nil)

				_, _, _, err = d.WriteBlock(blk)
				if err != nil {
					t.Fatalf("d.WriteBlock error %s",
						err)
				}

				if err = d.Flush(); err != nil {
					t.Fatalf("d.Flush() error %s", err)
				}

				sum := h.Sum(nil)

				if !bytes.Equal(sumOrig, sum) {
					t.Fatalf("error in block")
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
	const str = "=====foofoobarfoobar bartender======"
	tests := []struct {
		name string
		cfg  SeqConfig
	}{
		{
			name: "HashSequencer-3",
			cfg: &HSConfig{
				InputLen: 3,
				SBConfig: SBConfig{
					WindowSize: 8 << 20,
				},
			},
		},
		{
			name: "BackwardHashSequencer-3",
			cfg: &HSConfig{
				InputLen: 3,
				SBConfig: SBConfig{
					WindowSize: 8 << 20,
				},
			},
		},
		{
			name: "DoubleHashSequencer-3,6",
			cfg: &DHSConfig{
				InputLen1: 3,
				InputLen2: 6,
				SBConfig: SBConfig{
					WindowSize: 8 << 20,
				},
			},
		},
		{
			name: "BDHSequencer-3,6",
			cfg: &DHSConfig{
				InputLen1: 3,
				InputLen2: 6,
				SBConfig: SBConfig{
					WindowSize: 8 << 20,
				},
			},
		},
		{
			name: "GSASequencer",
			cfg: &GSASConfig{
				SBConfig: SBConfig{
					WindowSize: 8 << 20,
				},
			},
		},
		{
			name: "BucketHashSequencer",
			cfg: &BUHSConfig{
				SBConfig: SBConfig{
					WindowSize: 8 << 20,
				},
			},
		},
	}
	data := []byte(str)
	hd := sha256.New()
	hd.Write(data)
	sumData := hd.Sum(nil)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ws := newTestSequencer(t, tc.cfg)
			h := sha256.New()
			b := ws.Buffer()
			winSize := b.WindowSize
			d, err := NewDecoder(h, DConfig{
				WindowSize: winSize,
				MaxSize:    2 * winSize})
			if err != nil {
				t.Fatalf("NewDecoder error %s", err)
			}

			t.Logf("%q", data)
			s := Wrap(bytes.NewReader(data), ws)

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

				t.Logf("sequences: %+v", blk.Sequences)
				t.Logf("literals: %q", blk.Literals)

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
		cfg  SeqConfig
	}{
		{"HashSequencer-3", &HSConfig{
			InputLen: 3,
			HashBits: 15,
			SBConfig: SBConfig{
				WindowSize: 8 << 20,
			},
		}},
		{"HashSequencer-4", &HSConfig{
			InputLen: 4,
			HashBits: 15,
			SBConfig: SBConfig{
				WindowSize: 8 << 20,
			},
		}},
		{"HashSequencer-5", &HSConfig{
			InputLen: 5,
			HashBits: 15,
			SBConfig: SBConfig{
				WindowSize: 8 << 20,
			},
		}},
		{"HashSequencer-8", &HSConfig{
			InputLen: 8,
			SBConfig: SBConfig{
				WindowSize: 8 << 20,
			},
		}},
		{"BackwardHashSequencer-3", &HSConfig{
			InputLen: 3,
			HashBits: 15,
			SBConfig: SBConfig{
				WindowSize: 8 << 20,
			},
		}},
		{"BackwardHashSequencer-4", &HSConfig{
			InputLen: 4,
			HashBits: 15,
			SBConfig: SBConfig{
				WindowSize: 8 << 20,
			},
		}},
		{"BackwardHashSequencer-5", &HSConfig{
			InputLen: 5,
			HashBits: 15,
			SBConfig: SBConfig{
				WindowSize: 8 << 20,
			},
		}},
		{"BackwardHashSequencer-8", &HSConfig{
			InputLen: 8,
			HashBits: 15,
			SBConfig: SBConfig{
				WindowSize: 8 << 20,
			},
		}},
		{"DoubleHashSequencer-3,6", &DHSConfig{
			InputLen1: 3,
			InputLen2: 6,
			HashBits1: 15,
			HashBits2: 18,
			SBConfig: SBConfig{
				WindowSize: 8 << 20,
			},
		}},
		{"DoubleHashSequencer-4,6", &DHSConfig{
			InputLen1: 4,
			InputLen2: 6,
			HashBits1: 15,
			HashBits2: 18,
			SBConfig: SBConfig{
				WindowSize: 8 << 20,
			},
		}},
		{"BDHSequencer-3,6", &BDHSConfig{
			InputLen1: 3,
			InputLen2: 6,
			HashBits1: 15,
			HashBits2: 18,
			SBConfig: SBConfig{
				WindowSize: 8 << 20,
			},
		}},
		{"BDHSequencer-4,6", &BDHSConfig{
			InputLen1: 4,
			InputLen2: 6,
			HashBits1: 15,
			HashBits2: 18,
			SBConfig: SBConfig{
				WindowSize: 8 << 20,
			},
		}},
		{"GSASequencer", &GSASConfig{
			SBConfig: SBConfig{
				WindowSize: 8 << 20,
			},
		}},
		{"BUHSequencer-3-12", &BUHSConfig{
			InputLen:   3,
			HashBits:   18,
			BucketSize: 12,
			SBConfig: SBConfig{
				WindowSize: 8 << 20,
			},
		}},
		{"BUHSequencer-3-100", &BUHSConfig{
			InputLen:   3,
			HashBits:   18,
			BucketSize: 100,
			SBConfig: SBConfig{
				WindowSize: 8 << 20,
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
			hs, err := newHashSequencer(HSConfig{
				InputLen: 3,
				SBConfig: SBConfig{
					WindowSize: bm.winSize,
				},
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

func TestGSASSimple(t *testing.T) {
	const str = "=====foofoobarfoobar bartender===="
	const blockSize = 512

	var s greedySuffixArraySequencer
	if err := s.Init(GSASConfig{
		SBConfig: SBConfig{
			WindowSize: 1024,
			BlockSize:  blockSize,
		},
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
	if m != len(str) {
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

func verifyBlock(blk *Block, minMatchLen, windowSize int) error {
	if blk == nil {
		return errors.New("lz: blk is nil")
	}
	if minMatchLen < 1 {
		return fmt.Errorf("lz: minMatchLen=%d < 1", minMatchLen)
	}
	if windowSize < 1 {
		return fmt.Errorf("lu: windowSize=%d < 1", windowSize)
	}
	litLen := int64(0)
	for _, seq := range blk.Sequences {
		if seq.Offset == 0 {
			return errors.New("lz: offset is zero")
		}
		if int64(seq.Offset) > int64(windowSize) {
			return fmt.Errorf("lz: offset=%d > windowSize=%d",
				seq.Offset, windowSize)
		}
		if int64(seq.MatchLen) < int64(minMatchLen) {
			return fmt.Errorf("lz: matchLen=%d < minMatchLen=%d",
				seq.MatchLen, minMatchLen)
		}
		litLen += int64(seq.LitLen)
		if int64(seq.LitLen) > int64(len(blk.Literals)) {
			return fmt.Errorf("lz: litLen=%d > len(blk.Literals)=%d",
				seq.LitLen, len(blk.Literals))
		}
		if litLen > int64(len(blk.Literals)) {
			return fmt.Errorf(
				"lz: cumulative litLen=%d (liLen=%d) > len(blk.Literals)=%d",
				litLen, seq.LitLen, len(blk.Literals))
		}
	}
	return nil
}

type fuzzConfig struct {
	hs int

	len1   int
	len2   int
	hbits1 int
	hbits2 int

	shrinkSize int
	windowSize int
	bufferSize int
	blockSize  int

	data []byte
}

const (
	fzHS = iota + 1
	fzBHS
	fzDHS
	fzBDHS
	fzGSAS
	fzBUHS
)

func (fz *fuzzConfig) seqConfig() (cfg SeqConfig, err error) {
	switch fz.hs {
	case fzHS:
		return &HSConfig{
			InputLen: fz.len1,
			HashBits: fz.hbits1,
			SBConfig: SBConfig{
				ShrinkSize: fz.shrinkSize,
				WindowSize: fz.windowSize,
				BufferSize: fz.bufferSize,
				BlockSize:  fz.blockSize,
			},
		}, nil
	case fzBHS:
		return &BHSConfig{
			InputLen: fz.len1,
			HashBits: fz.hbits1,
			SBConfig: SBConfig{
				ShrinkSize: fz.shrinkSize,
				WindowSize: fz.windowSize,
				BufferSize: fz.bufferSize,
				BlockSize:  fz.blockSize,
			},
		}, nil
	case fzDHS:
		return &DHSConfig{
			InputLen1: fz.len1,
			HashBits1: fz.hbits1,
			InputLen2: fz.len1,
			HashBits2: fz.hbits1,
			SBConfig: SBConfig{
				ShrinkSize: fz.shrinkSize,
				WindowSize: fz.windowSize,
				BufferSize: fz.bufferSize,
				BlockSize:  fz.blockSize,
			},
		}, nil
	case fzBDHS:
		return &BDHSConfig{
			InputLen1: fz.len1,
			HashBits1: fz.hbits1,
			InputLen2: fz.len1,
			HashBits2: fz.hbits1,
			SBConfig: SBConfig{
				ShrinkSize: fz.shrinkSize,
				WindowSize: fz.windowSize,
				BufferSize: fz.bufferSize,
				BlockSize:  fz.blockSize,
			},
		}, nil
	case fzGSAS:
		return &GSASConfig{
			MinMatchLen: fz.len1,
			SBConfig: SBConfig{
				ShrinkSize: fz.shrinkSize,
				WindowSize: fz.windowSize,
				BufferSize: fz.bufferSize,
				BlockSize:  fz.blockSize,
			},
		}, nil
	case fzBUHS:
		return &BUHSConfig{
			InputLen:   fz.len1,
			BucketSize: fz.len2,
			HashBits:   fz.hbits1,
			SBConfig: SBConfig{
				ShrinkSize: fz.shrinkSize,
				WindowSize: fz.windowSize,
				BufferSize: fz.bufferSize,
				BlockSize:  fz.blockSize,
			},
		}, nil
	default:
		return nil, fmt.Errorf("lz: hs code %d not supported", fz.hs)
	}
}

func (fz *fuzzConfig) add(f *testing.F) {
	f.Add(fz.hs, fz.len1, fz.len2, fz.hbits1, fz.hbits2,
		fz.shrinkSize, fz.windowSize, fz.bufferSize, fz.blockSize,
		fz.data)
}

func newFuzzConfig(cfg SeqConfig, data []byte) (fz *fuzzConfig, err error) {
	switch c := cfg.(type) {
	case *HSConfig:
		return &fuzzConfig{
			hs:         fzHS,
			len1:       c.InputLen,
			hbits1:     c.HashBits,
			shrinkSize: c.ShrinkSize,
			windowSize: c.WindowSize,
			bufferSize: c.BufferSize,
			blockSize:  c.BlockSize,
			data:       data,
		}, nil
	case *BHSConfig:
		return &fuzzConfig{
			hs:         fzBHS,
			len1:       c.InputLen,
			hbits1:     c.HashBits,
			shrinkSize: c.ShrinkSize,
			windowSize: c.WindowSize,
			bufferSize: c.BufferSize,
			blockSize:  c.BlockSize,
			data:       data,
		}, nil
	case *DHSConfig:
		return &fuzzConfig{
			hs:         fzDHS,
			len1:       c.InputLen1,
			hbits1:     c.HashBits1,
			len2:       c.InputLen2,
			hbits2:     c.InputLen2,
			shrinkSize: c.ShrinkSize,
			windowSize: c.WindowSize,
			bufferSize: c.BufferSize,
			blockSize:  c.BlockSize,
			data:       data,
		}, nil
	case *BDHSConfig:
		return &fuzzConfig{
			hs:         fzBDHS,
			len1:       c.InputLen1,
			hbits1:     c.HashBits1,
			len2:       c.InputLen2,
			hbits2:     c.InputLen2,
			shrinkSize: c.ShrinkSize,
			windowSize: c.WindowSize,
			bufferSize: c.BufferSize,
			blockSize:  c.BlockSize,
			data:       data,
		}, nil
	case *GSASConfig:
		return &fuzzConfig{
			hs:         fzGSAS,
			len1:       c.MinMatchLen,
			shrinkSize: c.ShrinkSize,
			windowSize: c.WindowSize,
			bufferSize: c.BufferSize,
			blockSize:  c.BlockSize,
			data:       data,
		}, nil
	case *BUHSConfig:
		return &fuzzConfig{
			hs:         fzGSAS,
			len1:       c.InputLen,
			len2:       c.BucketSize,
			hbits1:     c.HashBits,
			shrinkSize: c.ShrinkSize,
			windowSize: c.WindowSize,
			bufferSize: c.BufferSize,
			blockSize:  c.BlockSize,
			data:       data,
		}, nil
	default:
		return nil, errors.New("lz: FuzzSequencer doesn't support configuration")
	}
}

func FuzzSequencer(f *testing.F) {
	tests := []struct {
		cfg  SeqConfig
		data []byte
	}{
		{
			&HSConfig{
				InputLen: 3,
				HashBits: 10,
				SBConfig: SBConfig{
					ShrinkSize: 16,
					WindowSize: 512,
					BufferSize: 512,
					BlockSize:  128,
				},
			},
			[]byte("HalloBallo"),
		},
	}
	for _, tc := range tests {
		fz, err := newFuzzConfig(tc.cfg, tc.data)
		if err != nil {
			f.Fatalf("newFuzzConfig error %s", err)
		}
		fz.add(f)
	}
	f.Fuzz(func(t *testing.T, hs, len1, len2, hbits1, hbits2,
		shrinkSize, windowSize, bufferSize, blockSize int, data []byte) {

		fz := &fuzzConfig{
			hs:         hs,
			len1:       len1,
			len2:       len2,
			hbits1:     hbits1,
			hbits2:     hbits2,
			shrinkSize: shrinkSize,
			windowSize: windowSize,
			bufferSize: bufferSize,
			blockSize:  blockSize,
			data:       data,
		}
		cfg, err := fz.seqConfig()
		if err != nil {
			t.Skip(err)
		}
		cfg.ApplyDefaults()
		if err = cfg.Verify(); err != nil {
			t.Skip(err)
		}
		seq, err := cfg.NewSequencer()
		if err != nil {
			t.Fatalf("cfg.NewSequencer error %s", err)
		}
		s := Wrap(bytes.NewReader(fz.data), seq)

		h := sha256.New()
		h.Write(fz.data)
		hsum := h.Sum(nil)
		h.Reset()

		d, err := NewDecoder(h, DConfig{
			WindowSize: fz.windowSize,
			MaxSize:    2 * fz.windowSize})
		if err != nil {
			t.Fatalf("NewDecoder error %s", err)
		}

		var blk Block
		for {
			_, err := s.Sequence(&blk, 0)
			if err != nil {
				if err == io.EOF {
					break
				}
				t.Fatalf("s.Sequence(blk) error %s", err)
			}
			err = verifyBlock(&blk, fz.len1, fz.windowSize)
			if err != nil {
				t.Fatalf("verifyBlock error %s", err)
			}

			_, _, _, err = d.WriteBlock(blk)
			if err != nil {
				t.Fatalf("d.WriteBlock error %s", err)
			}
		}

		if err = d.Flush(); err != nil {
			t.Fatalf("d.Flush error %s", err)
		}

		gsum := h.Sum(nil)
		if !bytes.Equal(gsum, hsum) {
			t.Fatalf("checksum got %x; want %x", gsum, hsum)
		}

	})
}
