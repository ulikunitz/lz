// Package lz provides encoders and decoders for LZ77 sequences. A sequence, as
// described in the [Zstandard specification], consists of a literal copy
// command followed by a match copy command. The literal copy command is
// described by the length in literal bytes to be copied and the match command
// consists of the distance of the match to copy and the length of the match in
// bytes.
//
// A [Sequencer] is an encoder that converts a byte stream into blocks of
// sequences. A [Decoder] converts the block of sequences into the original
// decompressed byte stream.
//
// The module provides multiple sequencers that provide different combinations
// of encoding speed  and compression ratios. Usually a slower sequencer will
// generate a better compression ratio.
//
// The [Decoder] slides the decompression window through a larger buffer
// implemented by [DecBuffer].
//
// [Zstandard specification]: https://github.com/facebook/zstd/blob/dev/doc/zstd_compression_format.md
package lz

import (
	"errors"
	"fmt"
	"reflect"
)

// Kilobytes and Megabyte defined as the more precise kibibyte and mebibyte.
const (
	_KiB = 1 << 10
	_MiB = 1 << 20
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

// Block stores sequences and literals. Note that literals that are not consumed
// by the Sequences slice need to be added to the end of the reconstructed data.
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

// ErrNoData indicates that no more data is available in the buffer.
var ErrNoData = errors.New("lz: no more data in buffer")

// Sequencer transforms byte streams into Lempel-Ziv sequences, that allow the
// reconstruction of the input data.
type Sequencer interface {
	// Update informs the sequencer that there is a change of the data
	// slice. There are three cases:
	//
	// 1. delta < 0: The sequencer is informed that this is complete new
	//    data.
	// 2. delta == 0: There should be additional data, but it is ok to
	//    provide the same data slice again.
	// 3. delta > 0: The data has been shifted down. The delta must be
	//    smaller than the old data slice provided.
	Update(data []byte, delta int)

	// Sequence finds Lempel-Ziv sequences. It returns the number of bytes
	// sequenced and the current position of window head in the provided data
	// slice. If no more data is available the method returns [fErrNoData].
	Sequence(blk *Block, flags int) (n int, err error)

	// Config returns the config generating this sequencer. This allows
	// upper level code to access the configuration information. So we don't
	// need to find a new mechanism for the configuration of buffer sizes
	// and shrink sizes.
	Config() SeqConfig
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

// BufferConfig reads the BufConfig from the sequencer configuration.
func BufferConfig(x SeqConfig) BufConfig {
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

func SetBufferConfig(x SeqConfig, bc BufConfig) {
	v := reflect.Indirect(reflect.ValueOf(x))
	setIVal(v, "ShrinkSize", bc.ShrinkSize)
	setIVal(v, "BufferSize", bc.BufferSize)
	setIVal(v, "WindowSize", bc.WindowSize)
	setIVal(v, "BlockSize", bc.BlockSize)
}

// BC gets the buffer configuration for the sequencer.
func BC(seq Sequencer) BufConfig {
	return BufferConfig(seq.Config())
}

// SeqConfig generates  new sequencer instances. Note that the sequencer doesn't
// use ShrinkSize and BufferSize directly but we added it here, so it can be
// used for the WriteSequencer which provides a WriteCloser interface.
type SeqConfig interface {
	NewSequencer() (s Sequencer, err error)
	ApplyDefaults()
	Verify() error
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
	maxSize := int64(maxUint32)
	if int64(maxInt) < maxSize {
		maxSize = maxInt
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

// ApplyDefaults sets the defaults for the various size values. The defaults are
// given below.
//
//	BufferSize:   8 MiB
//	ShrinkSize:  32 KiB (or half of BufferSize, if it is smaller than 64 KiB)
//	WindowSize: BufferSize
//	BlockSize:  128 KiB
func (cfg *BufConfig) ApplyDefaults() {
	if cfg.BufferSize == 0 {
		cfg.BufferSize = 8 * _MiB
	}
	if cfg.ShrinkSize == 0 {
		if cfg.BufferSize < 64*_KiB {
			cfg.ShrinkSize = cfg.BufferSize >> 1
		} else {
			cfg.ShrinkSize = 32 * _KiB
		}
	}
	if cfg.WindowSize == 0 {
		cfg.WindowSize = cfg.BufferSize
	}
	if cfg.BlockSize == 0 {
		cfg.BlockSize = 128 * _KiB
	}
}

// BufConfig returns the buffer configuration.
func (cfg *BufConfig) BufferConfig() BufConfig {
	return *cfg
}

// A matchFinder is used to find potential matches.
type MatchFinder interface {
	// Update informs the match finder to data changes in the data slice. If
	// delta is less than zero than complete new data is provided. If the
	// delta is positive data has been moved delta bytes down in the slice.
	// If delta is zero data has been added.
	Update(data []byte, delta int)

	// Process segment puts all hashes into the has table unless there is
	// not enough enough data at the end.
	ProcessSegment(a, b int)

	// AppendMatchOffsets looks for potential matches and adds the position
	// i to the finder.
	AppendMatchOffsets(m []uint32, i int) []uint32
}
