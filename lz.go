// SPDX-FileCopyrightText: © 2021 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

// Package lz supports encoding and decoding of LZ77 sequences. A sequence, as
// described in the [Zstandard specification], consists of a literal copy
// command followed by a match copy command. The literal copy command is
// described by the length in literal bytes to be copied, and the match command
// consists of the distance of the match to copy and the length of the match in
// bytes.
//
// A [Parser] converts a byte stream into blocks of sequences. The
// [Decoder] converts the block of sequences into the original
// decompressed byte stream.
//
// The module provides multiple parser implementations that offer different
// combinations of encoding speed and compression ratios. Usually, a slower
// parser will generate a better compression ratio.
//
// Parsers may use different matchers to provide their functionality. One
// Example is [greedyParser] which can use multiple Matcher implementations.
//
// The library supports the implementation of parsers outside of this package.
//
// [Zstandard specification]: https://github.com/facebook/zstd/blob/dev/doc/zstd_compression_format.md
package lz

import "io"

// Seq represents a single Lempel-Ziv 77 sequence describing a match,
// consisting of the offset, the length of the match, and the number of
// literals preceding the match. The Aux field can be used in upper
// layers to store additional information.
type Seq struct {
	LitLen   uint32
	MatchLen uint32
	Offset   uint32
	Aux      uint32
}

// Len returns the complete length of the sequence in bytes.
func (s Seq) Len() int64 {
	return int64(s.MatchLen) + int64(s.LitLen)
}

// Block stores sequences and literals. Note that the sequences stored in the
// Sequences slice might not consume the entire Literals slice. The remaining
// literal bytes must be added to the decoded text after all sequences have
// been decoded.
type Block struct {
	Sequences []Seq
	Literals  []byte
}

// Len computes the length of the block in bytes. It assumes that the sum of the
// literal lengths in the sequences does not exceed the length of the Literals
// byte slice.
func (b *Block) Len() int64 {
	n := int64(len(b.Literals))
	for _, s := range b.Sequences {
		n += int64(s.MatchLen)
	}
	return n
}

// Matcher is responsible to find matches or Literal bytes in the byte stream.
type Matcher interface {
	AppendEdges(q []Seq, n int) []Seq
	Skip(n int) (skipped int, err error)

	Prune(n int) int
	Write(p []byte) (n int, err error)
	ReadFrom(r io.Reader) (n int64, err error)

	ReadAt(p []byte, off int64) (n int, err error)
	ByteAt(off int64) (c byte, err error)

	Reset(data []byte) error
	Buf() *Buffer
}

// ParserFlags define optional parser behavior.
type ParserFlags int

const (
	// NoTrailingLiterals indicates that the parser should not generate
	// trailing literal bytes in the output.
	NoTrailingLiterals ParserFlags = 1 << iota
)

// Parser can parse the underlying byte stream into blocks of sequences.
type Parser interface {
	Parse(blk *Block, n int, flags ParserFlags) (parsed int, err error)

	Prune(n int) int
	Write(p []byte) (n int, err error)
	ReadFrom(r io.Reader) (n int64, err error)

	ReadAt(p []byte, off int64) (n int, err error)
	ByteAt(off int64) (c byte, err error)

	Reset(data []byte) error
}

type ParserOptions interface {
	SetWindowSize(s int)
	NewParser() (Parser, error)
}

type MatcherOptions interface {
	SetWindowSize(s int)
	NewMatcher() (Matcher, error)
}
