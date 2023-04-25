package lz2

import (
	"bytes"
	"os"
	"testing"
)

func TestWindow_Write(t *testing.T) {
	const file = "../testdata/enwik7"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error %s", file, err)
	}
	var w SeqWriter
	const winSize = 1024
	hcfg := HSConfig{
		WindowSize: winSize,
	}
	hcfg.ApplyDefaults()
	seq, err := hcfg.NewSequencer()
	if err != nil {
		t.Fatalf("hcfg.NewSequencer() error %s", err)
	}
	if err = w.Init(seq, nil); err != nil {
		t.Fatalf("w.Init(%d) error %s", winSize, err)
	}
	n, err := w.Write(data)
	if err != ErrFullBuffer {
		t.Fatalf("w.Write(data) return error %v; want %v",
			err, ErrFullBuffer)
	}
	if n != winSize {
		t.Fatalf("w.Write(data) wrote %d bytes; want %d",
			n, winSize)
	}
	if len(w.data) != winSize {
		t.Fatalf("len(w.data) is %d; want %d", len(w.data), winSize)
	}
	if cap(w.data) != winSize+7 {
		t.Fatalf("cap(w.data) is %d; want %d", cap(w.data), winSize+7)
	}
	if !bytes.Equal(w.data, data[:winSize]) {
		t.Fatalf("w.data doesn't equal data[:winSize]")
	}
}

func TestWindow_ReadFrom(t *testing.T) {
	const file = "../testdata/enwik7"
	f, err := os.Open(file)
	if err != nil {
		t.Fatalf("os.Open(%q) error %s", file, err)
	}
	defer f.Close()
	var w SeqWriter
	const winSize = 1024
	hcfg := HSConfig{
		WindowSize: winSize,
	}
	hcfg.ApplyDefaults()
	seq, err := hcfg.NewSequencer()
	if err != nil {
		t.Fatalf("hcfg.NewSequencer() error %s", err)
	}
	if err = w.Init(seq, nil); err != nil {
		t.Fatalf("w.Init(%d) error %s", winSize, err)
	}
	n, err := w.ReadFrom(f)
	if err != ErrFullBuffer {
		t.Fatalf("w.ReadFrom(f) returns error %v; want %v",
			err, ErrFullBuffer)
	}
	if n != winSize {
		t.Fatalf("w.ReadFrom(f) read %d bytes; want %d",
			n, winSize)
	}
	if len(w.data) != winSize {
		t.Fatalf("len(w.data) is %d; want %d", len(w.data), winSize)
	}
	if cap(w.data) != winSize+7 {
		t.Fatalf("cap(w.data) is %d; want %d", cap(w.data), winSize+7)
	}
	f.Close()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("os.ReadFile(f)")
	}
	if !bytes.Equal(w.data, data[:winSize]) {
		t.Fatalf("w.data doesn't equal data[:winSize]")
	}
}

func TestWindow_shrink(t *testing.T) {
	const file = "../testdata/enwik7"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error %s", file, err)
	}
	var w SeqWriter
	const winSize = 1024
	const shrinkSize = 256
	cfg := HSConfig{
		WindowSize: winSize,
		ShrinkSize: shrinkSize,
	}
	seq, err := cfg.NewSequencer()
	if err != nil {
		t.Fatalf("cfg.NewSequencer() error %s", err)
	}
	if err = w.Init(seq, nil); err != nil {
		t.Fatalf("w.Init(%d) error %s", winSize, err)
	}
	_, err = w.Write(data)
	if err != ErrFullBuffer {
		t.Fatalf("w.Write(data) return error %v; want %v",
			err, ErrFullBuffer)
	}

	w.w = winSize

	w.Shrink()
	if w.w != shrinkSize {
		t.Fatalf("w.shrink() returned %d; want %d", w.w, shrinkSize)
	}

	if len(w.data) != shrinkSize {
		t.Fatalf("len(w.data) is %d; want %d", len(w.data), shrinkSize)
	}
	if w.w != shrinkSize {
		t.Fatalf("w.w is %d; want %d", len(w.data), shrinkSize)
	}
	wantStart := int64(winSize - shrinkSize)
	if w.start != wantStart {
		t.Fatalf("w.start is %d; want %d", w.start, wantStart)
	}
}
