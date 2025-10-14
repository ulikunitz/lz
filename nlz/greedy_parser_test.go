package nlz

import "testing"

func TestGreedyParser(t *testing.T) {
	const str = "=====foofoobarfoobar bartender===="

	hashOptions := HashOptions{
		InputLen: 3,
		HashBits: 16,

		BufferSize:   64,
		WindowSize:   32,
		MinMatchSize: 3,
		MaxMatchSize: 64,
	}

	hm, err := NewHashMatcher(&hashOptions)
	if err != nil {
		t.Fatalf("NewHashMatcher: %v", err)
	}

	gpOptions := GreedyParserOptions{
		BlockSize: 128,
	}
	gp, err := NewGreedyParser(hm, &gpOptions)
	if err != nil {
		t.Fatalf("NewGreedyParser: %v", err)
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
	parsed, err := gp.Parse(&blk, len(str), 0)
	if err != nil {
		t.Fatalf("gp.Parse: %v", err)
	}
	if parsed != len(str) {
		t.Fatalf("gp.Parse returned parsed=%d; want %d", parsed, len(str))
	}

	decoderOpts := DecoderOptions{
		WindowSize: hashOptions.WindowSize,
		BufferSize: 2 * hashOptions.WindowSize,
	}
	d, err := NewDecoder(&decoderOpts)
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
	p := make([]byte, len(str))
	n, err = d.Read(p)
	if err != nil {
		t.Fatalf("d.Read: %v", err)
	}
	if n != len(str) {
		t.Fatalf("d.Read returned n=%d; want %d", n, len(str))
	}
	if string(p) != str {
		t.Fatalf("decoded string = %q; want %q", string(p), str)
	}
}
