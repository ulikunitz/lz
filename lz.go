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
// The package has a generic parser that needs a path finder, which selects the
// sequences for data block and a mapper that finds potential matches in the
// byte stream.
//
// For optimization we may provide custom implementations that integrate matcher
// or path finder into the parser and avoids the calling overhead through
// interfaces.
//
// The library supports the implementation of parsers outside of this package.
//
// [Zstandard specification]: https://github.com/facebook/zstd/blob/dev/doc/zstd_compression_format.md
package lz

import (
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// Seq represents a single Lempel-Ziv 77 sequence describing a match,
// consisting of the offset, the length of the match, and the number of
// literals preceding the match. The Aux field can be used in upper
// layers to store additional information. One use case is to store up to 4
// bytes of literals.
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
// of the literal lengths in the sequences is less than the bytes in the
// Literals field. If that is not the case an error is returned.
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
	Options() ParserOptions
}

// ParserOptions provides the configuration for a parser. PathFinder describes
// the algorithm to find a path through the different matches. The Mapper is
// used to find potential matches.
type ParserOptions struct {
	PathFinder string `json:",omitzero"`
	Mapper     string `json:",omitzero"`

	WindowSize    Size `json:",omitzero"`
	RetentionSize Size `json:",omitzero"`
	BufferSize    Size `json:",omitzero"`

	MinMatchLen int `json:",omitzero"`
	MaxMatchLen int `json:",omitzero"`
}

// SetDefaults sets the default values for the parser options if the field is
// zero or empty.
func (o *ParserOptions) SetDefaults() {
	if o.PathFinder == "" {
		o.PathFinder = "greedy"
	}
	if o.Mapper == "" {
		o.Mapper = "hash_4:16"
	}
	if o.BufferSize == 0 {
		o.BufferSize = 128 << 20
	}
	if o.RetentionSize == 0 {
		o.RetentionSize = min(o.BufferSize/4, 32<<10)
	}
	if o.WindowSize == 0 {
		o.WindowSize = o.BufferSize
	}
	if o.MinMatchLen == 0 {
		o.MinMatchLen = 3
	}
	if o.MaxMatchLen == 0 {
		o.MaxMatchLen = 273
	}
}

// Size is a specific type for handling data size parameters. It shortens the
// string representation. For instance 8 MiB are represented as "8M", 16 KiB as "16K", 2 GiB
// as "2G".
type Size int

// String returns the string representation of the size. It uses K, M, and G as suffixes for
// KiB, MiB, and GiB, respectively.
func (s Size) String() string {
	switch {
	case s == 0:
		return "0"
	case s%(1<<30) == 0:
		return fmt.Sprintf("%dG", s/(1<<30))
	case s%(1<<20) == 0:
		return fmt.Sprintf("%dM", s/(1<<20))
	case s%(1<<10) == 0:
		return fmt.Sprintf("%dK", s/(1<<10))
	default:
		return fmt.Sprintf("%d", s)
	}
}

// MarshalText returns the string representation of the size as byte slice. It
// is used by the JSON encoder.
func (s Size) MarshalText() ([]byte, error) {
	return []byte(s.String()), nil
}

var sizeRegexp = sync.OnceValue(func() *regexp.Regexp {
	return regexp.MustCompile(`^(\d+)([KMG]?)$`)
})

// parseSize parses the string representation of the Size type.
func parseSize(s string) (size Size, err error) {
	const msg = "lz: invalid size %q; must be in format <number>[K|M|G]"
	m := sizeRegexp().FindStringSubmatch(s)
	if m == nil {
		return 0, fmt.Errorf(msg, s)
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, fmt.Errorf(msg, s)
	}
	switch m[2] {
	case "K":
		n *= 1 << 10
	case "M":
		n *= 1 << 20
	case "G":
		n *= 1 << 30
	}
	return Size(n), nil
}

// UnmarshalText parses the string representation of the size and sets the value
// of s. It is used by the JSON decoder.
func (s *Size) UnmarshalText(text []byte) error {
	var err error
	*s, err = parseSize(string(text))
	return err
}

// NewParser creates a new parser for the provided options.
func NewParser(opts ParserOptions) (Parser, error) {
	opts.SetDefaults()
	return newGenericParser(opts)
}

// Matcher is responsible to find matches or Literal bytes in the byte stream.
// It is only relevant for the PathFinder.
type Matcher interface {
	// Edges returns the potential sequence at the current position.
	Edges(n int) []Seq

	// Skip is called to advance the current position by n bytes. An error
	// is only returned if the there are not enough bytes in the buffer.
	// Note that n can be negative, to allow to set the current position
	// backwards.
	Skip(n int) (skipped int, err error)

	// Parsable returns the number of bytes that are available in the buffer
	// for parsing.
	Parsable() int
}

// PathFinder implements the central Parse Function of the Parser.
type PathFinder interface {
	Parse(block *Block, n int, flags ParserFlags) (parsed int, err error)
	Reset()
}

// NewPathFinder creates a new PathFinder for the provided name of the algorithm
// and the Matcher.
func NewPathFinder(name string, m Matcher) (PathFinder, error) {
	switch name {
	case "greedy":
		return &GreedyPathFinder{matcher: m}, nil
	default:
		return nil, fmt.Errorf("lz: unknown path finder name %q", name)
	}
}

// Entry is returned by a Mapper for a found match. It provides the position i
// of the match in the Data slice of the buffer and v contains the leading 4
// bytes of the match to avoid a lookup in the buffer.
type Entry struct{ i, v uint32 }

// Mapper provides potential matches for a given position in the byte stream. Is
// it usually implemented by hash tables.
type Mapper interface {
	// InputLen returns the length of the input data into the table. We are
	// supporting length from 2 to 8 bytes.
	InputLen() int

	// Reset resets the internal state of the mapper.
	Reset()

	// Shift is called by the number of bytes pruned from the buffer and
	// provide the new extended buffer to the mapper.
	Shift(delta int)

	// Put adds all values between a and w to the mapper. We assume that
	// cap(p) >= len(p) + 7.
	Put(p []byte, a, w int) int

	// Get returns all candidate entries for the provided hash value. The
	// entry value v contains the all 4 bytes stored a position i.
	Get(v uint64) []Entry
}

// NewMapper creates a new Mapper for the provided name of the algorithm. The
// mappers supported are described below.
//
// hash_<inputLen>:<hashBits>: A hash table with the provided input length
// and hash bits. The input length is between 2 and 8 bytes, and the hash
// bits can be 24 bits at maximum.
func NewMapper(name string) (Mapper, error) {
	prefix, _, found := strings.Cut(name, "-")
	if !found {
		return nil, fmt.Errorf("lz: unknown mapper name %q", name)
	}
	switch prefix {
	case "hash":
		inputLen, hashBits, err := parseHashName(name)
		if err != nil {
			return nil, err
		}
		return newHash(inputLen, hashBits)
	}
	return nil, fmt.Errorf("lz: unknown mapper name %q", name)
}
