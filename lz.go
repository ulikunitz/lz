// Package lz supports encoding and decoding of LZ77 sequences. A sequence, as
// described in the [Zstandard specification], consists of a literal copy
// command followed by a match copy command. The literal copy command is
// described by the length in literal bytes to be copied and the match command
// consists of the distance of the match to copy and the length of the match in
// bytes.
//
// A [Sequencer] is an encoder that converts a byte stream into blocks of
// sequences. A [Decoder] converts the block of sequences into the original
// decompressed byte stream. We provide a Sequencer interface only supporting
// the Sequence interface.
//
// The actual basic Sequencer provided by the package support the SeqBuffer
// interface, which has methods for writing and reading from the buffer. A pure
// Sequencer is provided by the [Wrap function.
//
// The module provides multiple sequencer implementations that provide different
// combinations of encoding speed  and compression ratios. Usually a slower
// sequencer will generate a better compression ratio.
//
// The [Decoder] slides the decompression window through a larger buffer
// implemented by [DecBuffer].
//
// The library supports the implementation of Sequencers outside the package
// that can then be used by real compressors as provided by the
// [github.com/ulikunitz/xz] module.
//
// [Zstandard specification]: https://github.com/facebook/zstd/blob/dev/doc/zstd_compression_format.md
package lz

import (
	"errors"
	"fmt"
	"io"
	"reflect"
)

// Kilobytes and Megabyte defined as the more precise kibibyte and mebibyte.
const (
	kiB = 1 << 10
	miB = 1 << 20
)

// Seq represents a single Lempel-Ziv 77 Sequence describing a match,
// consisting of the offset, the length of the match and the number of
// literals preceding the match. The Aux field can be used on upper
// layers to store additional information.
type Seq struct {
	LitLen   uint32
	MatchLen uint32
	Offset   uint32
	Aux      uint32
}

// Block stores sequences and literals. Note that the sequences stores in the
// Sequences slice might not consume the whole Literals slice. They must be
// added to the decoded text after all the sequences have been decoded and their
// content added to the decoder buffer.
type Block struct {
	Sequences []Seq
	Literals  []byte
}

// Flags for the sequence function stored in the block structure.
const (
	// NoTrailingLiterals tells a sequencer that trailing literals don't
	// need to be included in the block.
	NoTrailingLiterals = 1 << iota
)

// ErrEmptyBuffer indicates that no more data is available in the buffer. It
// will be returned by the Sequence method of  [Sequencer].
var ErrEmptyBuffer = errors.New("lz: no more data in buffer")

// ErrFullBuffer indicates that the buffer is full. It will be returned by the
// Write and ReadFrom methods of the [Sequencer].
var ErrFullBuffer = errors.New("lz: buffer is full")

// Sequencer provides the basic interface of a Sequencer. It provides the
// functions provided by SeqBuffer.
type Sequencer interface {
	Sequence(blk *Block, flags int) (n int, err error)
	Reset(data []byte)
	Shrink() int
	SeqConfig() SeqConfig
	BufferConfig() BufConfig
	Write(p []byte) (n int, err error)
	ReadFrom(r io.Reader) (n int64, err error)
	ReadAt(p []byte, off int64) (n int, err error)
	ByteAt(off int64) (c byte, err error)
}

// BufConfig describes the various sizes relevant for the buffer. Note that
// ShrinkSize should be significantly smaller than BufferSize at most 50%. The
// WindowSize is independent of the BufferSize, but usually the BufferSize
// should be larger or equal the WindowSize. A typical BlockSize for instance
// for the ZStandard compression is 128 kByte and limits the largest match len.
type BufConfig struct {
	ShrinkSize int
	BufferSize int

	WindowSize int
	BlockSize  int
}

func iVal(v reflect.Value, name string) int {
	return int(v.FieldByName(name).Int())
}

// bufferConfig reads the BufConfig from the sequencer configuration.
func bufferConfig(x SeqConfig) BufConfig {
	v := reflect.Indirect(reflect.ValueOf(x))
	bc := BufConfig{
		ShrinkSize: iVal(v, "ShrinkSize"),
		BufferSize: iVal(v, "BufferSize"),
		WindowSize: iVal(v, "WindowSize"),
		BlockSize:  iVal(v, "BlockSize"),
	}
	return bc
}

func setIVal(v reflect.Value, name string, i int) {
	v.FieldByName(name).SetInt(int64(i))
}

func setBufferConfig(x SeqConfig, bc BufConfig) {
	v := reflect.Indirect(reflect.ValueOf(x))
	setIVal(v, "ShrinkSize", bc.ShrinkSize)
	setIVal(v, "BufferSize", bc.BufferSize)
	setIVal(v, "WindowSize", bc.WindowSize)
	setIVal(v, "BlockSize", bc.BlockSize)
}

// SeqConfig generates  new sequencer instances. Note that the sequencer doesn't
// use ShrinkSize and BufferSize directly but we added it here, so it can be
// used for the WriteSequencer which provides a WriteCloser interface.
type SeqConfig interface {
	NewSequence() (s Sequencer, err error)
	BufferConfig() BufConfig
}

// Methods to the types defined above.

// Len returns the complete length of the sequence.
func (s Seq) Len() int64 {
	return int64(s.MatchLen) + int64(s.LitLen)
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

// Verify checks the buffer configuration. Note that window size and block size
// are independent of the rest of the other sizes only the shrink size must be
// less than the buffer size.
func (cfg *BufConfig) Verify() error {
	// We are taking care of the margin for tha hash sequencers.
	maxSize := int64(maxUint32) - 7
	if int64(maxInt) < maxSize {
		maxSize = maxInt - 7
	}
	if !(1 <= cfg.BufferSize && int64(cfg.BufferSize) <= maxSize) {
		return fmt.Errorf("lz.BufferConfig.Verify: BufferSize=%d out of range [%d,%d]",
			cfg.BufferSize, 1, maxSize)
	}
	if !(0 <= cfg.ShrinkSize && cfg.ShrinkSize <= cfg.BufferSize) {
		return fmt.Errorf("lz.BufferConfig.Verify: ShrinkSize=%d out of range [0,BufferSize=%d]",
			cfg.ShrinkSize, cfg.BufferSize)
	}
	if !(0 <= cfg.WindowSize && int64(cfg.WindowSize) <= maxSize) {
		return fmt.Errorf("lz.BufferConfig.Verify: WindowSize=%d out of range [%d,%d]",
			cfg.WindowSize, 0, maxSize)
	}
	if !(1 <= cfg.BlockSize && int64(cfg.BlockSize) <= maxSize) {
		return fmt.Errorf("lz.BufferConfig.Verify: cfg.BLockSize=%d out of range [%d,%d]",
			cfg.BlockSize, 1, maxSize)
	}
	return nil
}

// SetDefaults sets the defaults for the various size values. The defaults are
// given below.
//
//	BufferSize:   8 MiB
//	ShrinkSize:  32 KiB (or half of BufferSize, if it is smaller than 64 KiB)
//	WindowSize: BufferSize
//	BlockSize:  128 KiB
func (cfg *BufConfig) SetDefaults() {
	if cfg.WindowSize == 0 {
		cfg.WindowSize = 8 * miB
	}
	if cfg.BufferSize == 0 {
		cfg.BufferSize = cfg.WindowSize
	}
	if cfg.ShrinkSize == 0 {
		if cfg.BufferSize < 64*kiB {
			cfg.ShrinkSize = cfg.BufferSize >> 1
		} else {
			cfg.ShrinkSize = 32 * kiB
		}
	}
	if cfg.BlockSize == 0 {
		cfg.BlockSize = 128 * kiB
	}
}

// BufConfig returns the buffer configuration.
func (cfg *BufConfig) BufferConfig() BufConfig {
	return *cfg
}
