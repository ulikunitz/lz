package lz

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"
)

func TestGreedyParser(t *testing.T) {
	const (
		blockSize = 128
		str       = "=====foofoobarfoobar bartender===="
	)

	opts := &GreedyParserOptions{
		MatcherOptions: &GenericMatcherOptions{
			MinMatchLen: 3,
			MaxMatchLen: 64,
			MapperOptions: &HashOptions{
				InputLen: 3,
				HashBits: 16,
			},
		},
	}
	const winSize = 32
	p, err := opts.NewParser(winSize, winSize, 2*winSize)
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
	parsed, err := gp.Parse(&blk, blockSize, 0)
	if err != nil {
		t.Fatalf("gp.Parse: %v", err)
	}
	if parsed != len(str) {
		t.Fatalf("gp.Parse returned parsed=%d; want %d", parsed, len(str))
	}

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


func TestGreedyParserOptionsJSON(t *testing.T) {
	opts := &GreedyParserOptions{
		MatcherOptions: &GenericMatcherOptions{
			MinMatchLen:   4,
			MaxMatchLen:   32,
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

	c, err := ParseJSON(data)
	if err != nil {
		t.Fatalf("UnmarshalJSONOptions: %v", err)
	}

	gOpts, ok := c.(*GreedyParserOptions)
	if !ok {
		t.Fatalf(
			"UnmarshalJSONOptions returned type %T; want *GreedyParserOptions",
			c)
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
		const (
			blockSize = 128
			winSize   = 200
		)
		pOpts := &GreedyParserOptions{
			MatcherOptions: &GenericMatcherOptions{
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

		p, err := pOpts.NewParser(winSize, winSize, 2*winSize)
		if err != nil {
			t.Fatalf("NewGreedyParser: %v", err)
		}
		d, err := dOpts.NewDecoder()
		if err != nil {
			t.Fatalf("NewDecoder: %v", err)
		}

		r := bytes.NewReader(data)
		w := new(bytes.Buffer)

		// TODO: Call ReadFrom(r) only if p.Parse returns ErrEndOfBuffer.
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
				k, err := p.Parse(&blk, blockSize, 0)
				t.Logf("p.Parse: %d bytes", k)
				if err != nil {
					if err != ErrEndOfBuffer {
						t.Fatalf("p.Parse: %v", err)
					}
					if k == 0 {
						break
					}
				}

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
