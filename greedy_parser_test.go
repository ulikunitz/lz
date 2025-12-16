package lz

import "testing"

func TestGreedyParser(t *testing.T) {
	const str = "=====foofoobarfoobar bartender===="

	opts := &GreedyParserOptions{
		BlockSize: 128,
		Matcher: &GenericMatcherOptions{
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
		Matcher: &GenericMatcherOptions{
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
