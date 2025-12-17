package lz

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"
)

func TestGreedyParser(t *testing.T) {
	const str = "=====foofoobarfoobar bartender===="

	opts := &GreedyParserOptions{
		BlockSize: 128,
		MatcherOptions: &GenericMatcherOptions{
			BufferSize:  64,
			WindowSize:  32,
			MinMatchLen: 3,
			MaxMatchLen: 64,
			MapperOptions: &HashOptions{
				InputLen: 3,
				HashBits: 16,
			},
		},
	}
	p, err := opts.NewParser()
	if err != nil {
		t.Fatalf("NewGreedyParser: %v", err)
	}
	gp, ok := p.(*greedyParser)
	if !ok {
		t.Fatalf("NewGreedyParser returned type %T; want *greedyParser", p)
	}

	buf := gp.Buf()
	n, err := buf.Write([]byte(str))
	if err != nil {
		t.Fatalf("buf.Write: %v", err)
	}
	if n != len(str) {
		t.Fatalf("buf.Write returned n=%d; want %d", n, len(str))
	}

	var blk Block
	parsed, err := gp.Parse(&blk, 0)
	if err != nil {
		t.Fatalf("gp.Parse: %v", err)
	}
	if parsed != len(str) {
		t.Fatalf("gp.Parse returned parsed=%d; want %d", parsed, len(str))
	}

	winSize := WindowSize(opts)
	decoderOpts := DecoderOptions{
		WindowSize: winSize,
		BufferSize: 2 * winSize,
	}
	d, err := decoderOpts.NewDecoder()
	if err != nil {
		t.Fatalf("NewDecoder: %v", err)
	}
	n, err = d.WriteBlock(&blk)
	if err != nil {
		t.Fatalf("d.WriteBlock: %v", err)
	}
	if n != len(str) {
		t.Fatalf("d.WriteBlock returned n=%d; want %d", n, len(str))
	}
	q := make([]byte, len(str))
	n, err = d.Read(q)
	if err != nil {
		t.Fatalf("d.Read: %v", err)
	}
	if n != len(str) {
		t.Fatalf("d.Read returned n=%d; want %d", n, len(str))
	}
	if string(q) != str {
		t.Fatalf("decoded string = %q; want %q", string(q), str)
	}
}

func TestWindowSize(t *testing.T) {
	opts := &GreedyParserOptions{
		BlockSize: 128,
		MatcherOptions: &GenericMatcherOptions{
			BufferSize:  64,
			WindowSize:  32,
			MinMatchLen: 3,
			MaxMatchLen: 64,
			MapperOptions: &HashOptions{
				InputLen: 3,
				HashBits: 16,
			},
		},
	}
	n := WindowSize(opts)
	if n != 32 {
		t.Fatalf("GetWindowSize returned %d; want 32", n)
	}
	if err := SetWindowSize(opts, 48); err != nil {
		t.Fatalf("SetWindowSize: %v", err)
	}
	n = WindowSize(opts)
	if n != 48 {
		t.Fatalf("GetWindowSize returned %d; want 48", n)
	}
}

func TestBufferSize(t *testing.T) {
	opts := &GreedyParserOptions{
		BlockSize: 128,
		MatcherOptions: &GenericMatcherOptions{
			BufferSize:  64,
			WindowSize:  32,
			MinMatchLen: 3,
			MaxMatchLen: 64,
			MapperOptions: &HashOptions{
				InputLen: 3,
				HashBits: 16,
			},
		},
	}
	n := BufferSize(opts)
	if n != 64 {
		t.Fatalf("GetBufferSize returned %d; want 64", n)
	}
	if err := SetBufferSize(opts, 96); err != nil {
		t.Fatalf("SetBufferSize: %v", err)
	}
	n = BufferSize(opts)
	if n != 96 {
		t.Fatalf("GetBufferSize returned %d; want 96", n)
	}
}

func TestGreedyParserOptionsJSON(t *testing.T) {
	opts := &GreedyParserOptions{
		BlockSize: 256,
		MatcherOptions: &GenericMatcherOptions{
			BufferSize:  128,
			WindowSize:  64,
			MinMatchLen: 4,
			MaxMatchLen: 32,
			MapperOptions: &HashOptions{
				InputLen: 4,
				HashBits: 20,
			},
		},
	}

	data, err := json.MarshalIndent(opts, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent: %v", err)
	}

	t.Logf("GreedyParserOptions JSON:\n%s\n", data)

	c, err := UnmarshalJSONOptions(data)
	if err != nil {
		t.Fatalf("UnmarshalJSONOptions: %v", err)
	}

	gOpts, ok := c.(*GreedyParserOptions)
	if !ok {
		t.Fatalf(
			"UnmarshalJSONOptions returned type %T; want *GreedyParserOptions",
			c)
	}

	if gOpts.BlockSize != opts.BlockSize {
		t.Fatalf(
			"BlockSize = %d; want %d",
			gOpts.BlockSize, opts.BlockSize)
	}

	mOpts, ok := gOpts.MatcherOptions.(*GenericMatcherOptions)
	if !ok {
		t.Fatalf(
			"Matcher type = %T; want *GenericMatcherOptions",
			gOpts.MatcherOptions)
	}

	origMOpts, ok := opts.MatcherOptions.(*GenericMatcherOptions)
	if !ok {
		t.Fatalf(
			"Original Matcher type = %T; want *GenericMatcherOptions",
			opts.MatcherOptions)
	}

	if mOpts.BufferSize != origMOpts.BufferSize {
		t.Fatalf(
			"BufferSize = %d; want %d",
			mOpts.BufferSize, origMOpts.BufferSize)
	}
	if mOpts.WindowSize != origMOpts.WindowSize {
		t.Fatalf(
			"WindowSize = %d; want %d",
			mOpts.WindowSize, origMOpts.WindowSize)
	}
	if mOpts.MinMatchLen != origMOpts.MinMatchLen {
		t.Fatalf(
			"MinMatchLen = %d; want %d",
			mOpts.MinMatchLen, origMOpts.MinMatchLen)
	}
	if mOpts.MaxMatchLen != origMOpts.MaxMatchLen {
		t.Fatalf(
			"MaxMatchLen = %d; want %d",
			mOpts.MaxMatchLen, origMOpts.MaxMatchLen)
	}

	mapperOpts, ok := mOpts.MapperOptions.(*HashOptions)
	if !ok {
		t.Fatalf(
			"MapperOptions type = %T; want *HashOptions",
			mOpts.MapperOptions)
	}

	origMapperOpts, ok := origMOpts.MapperOptions.(*HashOptions)
	if !ok {
		t.Fatalf(
			"Original MapperOptions type = %T; want *HashOptions",
			origMOpts.MapperOptions)
	}

	if mapperOpts.InputLen != origMapperOpts.InputLen {
		t.Fatalf(
			"InputLen = %d; want %d",
			mapperOpts.InputLen, origMapperOpts.InputLen)
	}
	if mapperOpts.HashBits != origMapperOpts.HashBits {
		t.Fatalf(
			"HashBits = %d; want %d",
			mapperOpts.HashBits, origMapperOpts.HashBits)
	}
}

func FuzzGreedyParser(f *testing.F) {
	f.Add([]byte("a"))
	f.Add([]byte{})
	f.Add([]byte("abcabcabcabcabcabcabcabc"))
	f.Add([]byte("aaaaaaaaaaaaaaaaaaaaaaaaaa"))
	f.Fuzz(func(t *testing.T, data []byte) {
		const winSize = 200
		pOpts := &GreedyParserOptions{
			BlockSize: 128,
			MatcherOptions: &GenericMatcherOptions{
				BufferSize:  256,
				WindowSize:  winSize,
				MinMatchLen: 3,
				MaxMatchLen: 64,
				MapperOptions: &HashOptions{
					InputLen: 3,
					HashBits: 16,
				},
			},
		}
		dOpts := &DecoderOptions{
			WindowSize: winSize,
			BufferSize: 2 * winSize,
		}

		p, err := pOpts.NewParser()
		if err != nil {
			t.Fatalf("NewGreedyParser: %v", err)
		}
		d, err := dOpts.NewDecoder()
		if err != nil {
			t.Fatalf("NewDecoder: %v", err)
		}

		r := bytes.NewReader(data)
		w := new(bytes.Buffer)

		var blk Block
		moreData := true
		for moreData {
			k, err := p.ReadFrom(r)
			t.Logf("p.ReadFrom: %d bytes", k)
			if err != nil && err != ErrFullBuffer {
				t.Fatalf("p.ReadFrom: %v", err)
			}
			moreData = err == ErrFullBuffer

			for {
				k, err := p.Parse(&blk, 0)
				t.Logf("p.Parse: %d bytes", k)
				if err != nil {
					if err != ErrEndOfBuffer {
						t.Fatalf("p.Parse: %v", err)
					}
					if k == 0 {
						break
					}
				}
				k = p.Prune(winSize)
				t.Logf("p.Prune: %d bytes", k)

				k, err = d.WriteBlock(&blk)
				if err != nil {
					t.Fatalf("d.WriteBlock: %v", err)
				}
				t.Logf("d.WriteBlock: %d bytes", k)

				l, err := io.Copy(w, d)
				if err != nil {
					t.Fatalf("io.Copy: %v", err)
				}
				t.Logf("io.Copy: %d bytes", l)
			}
		}

		decoded := w.Bytes()
		if !bytes.Equal(decoded, data) {
			t.Fatalf(
				"decoded data does not match original\noriginal: %q\ndecoded:  %q",
				data, decoded)
		}
	})
}
