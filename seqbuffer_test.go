package lz

import (
	"bytes"
	"os"
	"testing"
)

func TestWindow_Write(t *testing.T) {
	const file = "testdata/enwik7"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error %s", file, err)
	}
	var w SeqBuffer
	const winSize = 1024
	cfg := SBConfig{
		WindowSize: winSize,
	}
	if err = w.Init(cfg); err != nil {
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
	const file = "testdata/enwik7"
	f, err := os.Open(file)
	if err != nil {
		t.Fatalf("os.Open(%q) error %s", file, err)
	}
	defer f.Close()
	var w SeqBuffer
	const winSize = 1024
	cfg := SBConfig{
		WindowSize: winSize,
	}
	if err = w.Init(cfg); err != nil {
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
	const file = "testdata/enwik7"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error %s", file, err)
	}
	var w SeqBuffer
	const winSize = 1024
	const shrinkSize = 256
	cfg := SBConfig{
		WindowSize: winSize,
		ShrinkSize: shrinkSize,
	}
	if err = w.Init(cfg); err != nil {
		t.Fatalf("w.Init(%d) error %s", winSize, err)
	}
	_, err = w.Write(data)
	if err != ErrFullBuffer {
		t.Fatalf("w.Write(data) return error %v; want %v",
			err, ErrFullBuffer)
	}

	w.w = winSize

	w.shrink()
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

func TestSetWindowSize(t *testing.T) {
	params := Params{WindowSize: 8 << 20}
	params.ApplyDefaults()
	c, err := Config(params)
	if err != nil {
		t.Fatalf("Config(%+v) error %s", params, err)
	}
	sbCfg := c.BufferConfig()
	sbCfg.SetWindowSize(4096)
	t.Logf("sbCfg: %+v", sbCfg)
	if err := sbCfg.Verify(); err != nil {
		t.Fatalf("sbCfg.Verify error %s", err)
	}
}
