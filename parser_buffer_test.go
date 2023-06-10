// SPDX-FileCopyrightText: © 2021 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

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
	var w ParserBuffer
	const winSize = 1024
	cfg := BufConfig{
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
	if len(w.Data) != winSize {
		t.Fatalf("len(w.Data) is %d; want %d", len(w.Data), winSize)
	}
	if cap(w.Data) < winSize+7 {
		t.Fatalf("cap(w.Data) is %d; want >= %d", cap(w.Data), winSize+7)
	}
	if !bytes.Equal(w.Data, data[:winSize]) {
		t.Fatalf("w.Data doesn't equal data[:winSize]")
	}
}

func TestWindow_ReadFrom(t *testing.T) {
	const file = "testdata/enwik7"
	f, err := os.Open(file)
	if err != nil {
		t.Fatalf("os.Open(%q) error %s", file, err)
	}
	defer f.Close()
	var w ParserBuffer
	const winSize = 1024
	cfg := BufConfig{
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
	if len(w.Data) != winSize {
		t.Fatalf("len(w.Data) is %d; want %d", len(w.Data), winSize)
	}
	if cap(w.Data) != winSize+7 {
		t.Fatalf("cap(w.Data) is %d; want %d", cap(w.Data), winSize+7)
	}
	f.Close()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("os.ReadFile(f)")
	}
	if !bytes.Equal(w.Data, data[:winSize]) {
		t.Fatalf("w.Data doesn't equal data[:winSize]")
	}
}

func TestWindow_shrink(t *testing.T) {
	const file = "testdata/enwik7"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error %s", file, err)
	}
	var w ParserBuffer
	const winSize = 1024
	const shrinkSize = 256
	cfg := BufConfig{
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

	w.W = winSize

	w.Shrink()
	if w.W != shrinkSize {
		t.Fatalf("w.shrink() returned %d; want %d", w.W, shrinkSize)
	}

	if len(w.Data) != shrinkSize {
		t.Fatalf("len(w.Data) is %d; want %d", len(w.Data), shrinkSize)
	}
	if w.W != shrinkSize {
		t.Fatalf("w.W is %d; want %d", len(w.Data), shrinkSize)
	}
	wantOff := int64(winSize - shrinkSize)
	if w.Off != wantOff {
		t.Fatalf("w.Off is %d; want %d", w.Off, wantOff)
	}
}
