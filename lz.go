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

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

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

// LenCheck computes the length of the block in bytes and verifies that the sum
// of the literal lengths in the sequences is less than the bytes in the Literals
// field. If that is not the case an error is returned.
func (b *Block) LenCheck() (n int64, err error) {
	litSum := int64(0)
	matchLen := int64(0)
	for _, s := range b.Sequences {
		litSum += int64(s.LitLen)
		matchLen += int64(s.MatchLen)
	}

	litLen := int64(len(b.Literals))
	if litSum > litLen {
		return 0, fmt.Errorf(
			"lz: block sequence literal lengths %d > literals length %d",
			litSum, litLen)
	}
	return litLen + matchLen, nil
}

// ParserFlags define optional parser behavior.
type ParserFlags int

const (
	// NoTrailingLiterals indicates that the parser should not generate
	// trailing literal bytes in the output.
	NoTrailingLiterals ParserFlags = 1 << iota
)

// Parser provides the possibility to parse a byte stream into LZ77 sequences.
type Parser interface {
	// Parse up to block size bytes from the internal buffer and provides
	// the sequences in the block structure. While slices will be reused,
	// not old information will be maintained.
	Parse(blk *Block, n int, flags ParserFlags) (parsed int, err error)

	// Write writes data into the internal buffer.
	Write(p []byte) (n int, err error)

	// ReadFrom reads data from the provided reader into the internal
	// buffer.
	ReadFrom(r io.Reader) (n int64, err error)

	// ReadAt reads len(p) bytes from the internal buffer at offset off.
	ReadAt(p []byte, off int64) (n int, err error)

	// ByteAt returns the byte at offset off in the internal buffer.
	ByteAt(off int64) (c byte, err error)

	// Reset resets the internal buffer to the provided data.
	Reset(data []byte) error

	// Options returns the options used to create the parser.
	Options() Configurator
}

// Configurator creates a parser. Usually an Options type implements the
// interface.
type Configurator interface {
	NewParser(windowSize, retentionSize, bufferSize int) (Parser, error)
}

// Matcher is responsible to find matches or Literal bytes in the byte stream.
type Matcher interface {
	Edges(n int) []Seq
	Skip(n int) (skipped int, err error)

	Write(p []byte) (n int, err error)
	ReadFrom(r io.Reader) (n int64, err error)

	ReadAt(p []byte, off int64) (n int, err error)
	ByteAt(off int64) (c byte, err error)

	Reset(data []byte) error

	Buf() *Buffer

	Options() MatcherConfigurator
}

// MatcherConfigurator creates a matcher, usually an Options type implements
// the interface.
type MatcherConfigurator interface {
	NewMatcher(windowSize, retentionSize, bufferSize int) (Matcher, error)
}

// UnmarshalJSONMatcherOptions unmarshals matcher options from JSON data. The
// function looks first for the MatcherType field to determine the type of
// matcher to create.
func UnmarshalJSONMatcherOptions(data []byte) (MatcherConfigurator, error) {
	var d struct{ MatcherType string }
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, err
	}
	switch d.MatcherType {
	case "generic":
		var opts GenericMatcherOptions
		if err := json.Unmarshal(data, &opts); err != nil {
			return nil, err
		}
		return &opts, nil
	default:
		return nil, errors.New(
			"lz: unknown matcher type: " + d.MatcherType)
	}
}

// Entry is returned by a Mapper for a found match.
type Entry struct{ i, v uint32 }

// Mapper will be typically implemented by hash tables.
//
// The Put method returns the number of trailing bytes that could not be hashed.
// Shift is called, when n bytes have been pruned from the buffer.
type Mapper interface {
	InputLen() int
	Reset()
	// Shift is called by the number of bytes pruned from the buffer.
	Shift(delta int)
	Put(a, w int, p []byte) int

	// Get returns all candidate entries for the provided hash value. The
	// entry value v contains the all 4 bytes stored a position i.
	Get(v uint64) []Entry
}

// MapperConfigurator creates a mapper, usually an Options type implements this
// function.
type MapperConfigurator interface {
	NewMapper() (Mapper, error)
}

// UnmarshalJSONMapperOptions unmarshals mapper options from JSON data. The
// function looks first for the MapperType field to determine the type of mapper
// to create.
func UnmarshalJSONMapperOptions(data []byte) (MapperConfigurator, error) {
	var d struct{ MapperType string }
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, err
	}
	switch d.MapperType {
	case "hash":
		var opts HashOptions
		if err := json.Unmarshal(data, &opts); err != nil {
			return nil, err
		}
		return &opts, nil
	default:
		return nil, errors.New(
			"lz: unknown mapper type: " + d.MapperType)
	}
}

// ParseJSON unmarshals parser options from JSON data.
func ParseJSON(data []byte) (Configurator, error) {
	var d struct{ Type string }
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, err
	}
	switch d.Type {
	case "greedy":
		var opts GreedyParserOptions
		if err := json.Unmarshal(data, &opts); err != nil {
			return nil, err
		}
		return &opts, nil
	default:
		return nil, errors.New(
			"lz: unknown parser type: " + d.Type)
	}
}
