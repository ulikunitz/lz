// SPDX-FileCopyrightText: © 2021 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

// Package lz supports encoding and decoding of LZ77 sequences. A sequence, as
// described in the [Zstandard specification], consists of a literal copy
// command followed by a match copy command. The literal copy command is
// described by the length in literal bytes to be copied and the match command
// consists of the distance of the match to copy and the length of the match in
// bytes.
//
// A [Parser] is an encoder that converts a byte stream into blocks of
// sequences. A [decoder] converts the block of sequences into the original
// decompressed byte stream.
//
// The actual basic Parser provided by the package support the SeqBuffer
// interface, which has methods for writing and reading from the buffer.
//
// The module provides multiple parser implementations that provide different
// combinations of encoding speed  and compression ratios. Usually a slower
// parser will generate a better compression ratio.
//
// The library supports the implementation of parsers outside of this package
// that can then be used by real compressors as provided by the
// [github.com/ulikunitz/xz] module.
//
// [Zstandard specification]: https://github.com/facebook/zstd/blob/dev/doc/zstd_compression_format.md
package lz

import (
	"errors"
	"io"
)

// Seq represents a single Lempel-Ziv 77 Parse describing a match,
// consisting of the offset, the length of the match and the number of
// literals preceding the match. The Aux field can be used on upper
// layers to store additional information.
type Seq struct {
	LitLen   uint32
	MatchLen uint32
	Offset   uint32
	Aux      uint32
}

// Len returns the complete length of the sequence.
func (s Seq) Len() int64 {
	return int64(s.MatchLen) + int64(s.LitLen)
}

// Block stores sequences and literals. Note that the sequences stores in the
// Sequences slice might not consume the whole Literals slice. They must be
// added to the decoded text after all the sequences have been decoded and their
// content added to the decoder buffer.
type Block struct {
	Sequences []Seq
	Literals  []byte
}

// Len computes the length of the block in bytes. It assumes that the sum of the
// literal lengths in the sequences doesn't exceed that length of the Literals
// byte slice.
func (b *Block) Len() int64 {
	n := int64(len(b.Literals))
	for _, s := range b.Sequences {
		n += int64(s.MatchLen)
	}
	return n
}

// Flags for the sequence function stored in the block structure.
const (
	// NoTrailingLiterals tells a parser that trailing literals don't
	// need to be included in the block.
	NoTrailingLiterals = 1 << iota
)

// ErrEmptyBuffer indicates that no more data is available in the buffer. It
// will be returned by the Parse method of [Parser].
var ErrEmptyBuffer = errors.New("lz: no more data in buffer")

// ErrFullBuffer indicates that the buffer is full. It will be returned by the
// Write and ReadFrom methods of the [Parser].
var ErrFullBuffer = errors.New("lz: buffer is full")

// Parser provides the basic interface of a Parser. Most of the functions are
// provided by the underlying [Buffer].
type Parser interface {
	Parse(blk *Block, flags int) (n int, err error)
	Reset(data []byte) error
	Shrink() int
	ParserConfig() ParserConfig
	BufferConfig() BufConfig
	Write(p []byte) (n int, err error)
	ReadFrom(r io.Reader) (n int64, err error)
	ReadAt(p []byte, off int64) (n int, err error)
	ByteAt(off int64) (c byte, err error)
}
