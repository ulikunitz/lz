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

	const winSize = 32
	opts := ParserOptions{
		PathFinder: "greedy",
		Mapper:     "hash_3:16",

		WindowSize:    winSize,
		RetentionSize: winSize,
		BufferSize:    2 * winSize,
	}

	p, err := NewParser(opts)
	if err != nil {
		t.Fatalf("NewParser: %v", err)
	}
	gp, ok := p.(*genericParser)
	if !ok {
		t.Fatalf(
			"NewGreedyParser returned type %T; want *genericParser",
			p)
	}

	buf := &gp.Buffer
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
	t.Logf("Literals: %q", blk.Literals)
	t.Logf("Sequences: %v", blk.Sequences)

	decoderOpts := DecoderOptions{
		WindowSize: winSize,
		BufferSize: 2 * winSize,
	}
	d, err := NewDecoder(decoderOpts)
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

func TestParserOptionsJSON(t *testing.T) {
	const winSize = 32
	opts := ParserOptions{
		PathFinder: "greedy",
		Mapper:     "hash_3:16",

		WindowSize:    winSize,
		RetentionSize: winSize,
		BufferSize:    2 * winSize,

		MinMatchLen: 4,
		MaxMatchLen: 32,
	}

	data, err := json.MarshalIndent(opts, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent: %v", err)
	}

	t.Logf("ParserOptions JSON:\n%s\n", data)

	var o ParserOptions
	c := &o
	err = json.Unmarshal(data, c)
	if err != nil {
		t.Fatalf("UnmarshalJSONOptions: %v", err)
	}

	if o != opts {
		t.Fatalf(
			"UnmarshalJSONOptions returned options = %+v; want %+v",
			o, opts)
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
		opts := ParserOptions{
			PathFinder:    "greedy",
			Mapper:        "hash_3:16",
			MinMatchLen:   3,
			MaxMatchLen:   64,
			WindowSize:    winSize,
			RetentionSize: winSize,
			BufferSize:    2 * winSize,
		}
		dOpts := &DecoderOptions{
			WindowSize: winSize,
			BufferSize: 2 * winSize,
		}

		p, err := NewParser(opts)
		if err != nil {
			t.Fatalf("NewParser: %v", err)
		}
		d, err := NewDecoder(*dOpts)
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
