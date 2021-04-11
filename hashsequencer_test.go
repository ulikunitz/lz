package lz

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestHashSequencerSimple(t *testing.T) {
	const str = "=====foofoobarfoobar bartender===="

	var s HashSequencer
	if err := s.Init(HashSequencerConfig{
		WindowSize:  1024,
		ShrinkSize:  1024,
		InputLen:    3,
		MinMatchLen: 2,
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
	n, err = s.Sequence(&blk, 1024, 0)
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
	if err := d.Init(&buf, 1024); err != nil {
		t.Fatalf("dw.Init(%d) error %s", 1024, err)
	}
	k, l, m, err := d.WriteBlock(&blk)
	if err != nil {
		t.Fatalf("dw.WriteBlock(&blk) error %s", err)
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
		t.Fatalf("dw.WriteBlock(&blk) returned %d; want %d bytes",
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

func TestWrapHashSequencer(t *testing.T) {
	const (
		windowSize = 1024
		blockSize  = 512
		str        = "=====foofoobarfoobar bartender===="
	)

	ws, err := NewHashSequencer(HashSequencerConfig{
		WindowSize:  windowSize,
		ShrinkSize:  windowSize,
		InputLen:    3,
		MinMatchLen: 2,
	})
	if err != nil {
		t.Fatalf("NewHashSequencer error %s", err)
	}
	s := WrapReader(strings.NewReader(str), ws)

	var builder strings.Builder
	var decoder Decoder
	decoder.Init(&builder, windowSize)

	var blk Block
	for {
		if _, err := s.Sequence(&blk, blockSize, 0); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("s.Sequence error %s", err)
		}
		if _, _, _, err := decoder.WriteBlock(&blk); err != nil {
			t.Fatalf("decoder.WriteBlock error %s", err)
		}
	}
	if err := decoder.Flush(); err != nil {
		t.Fatalf("decoder.Flush error %s", err)
	}

	g := builder.String()
	if g != str {
		t.Fatalf("got string %q; want %q", g, str)
	}
}
