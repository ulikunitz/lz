// Package lz provides encoders and decoders for LZ77 sequences. A
// sequence, as described in the zstd specification, describes a number
// of literal bytes and a match.
//
// A Sequencer is an encoder that converts a byte stream into blocks of
// sequences. A Decoder converts the block of sequences into the
// original decompressed byte stream. A wrapped Sequencer reads the byte
// stream from a reader. The sequencers are provided here seperately
// because they are more efficient for encoding byte slices directly.
//
// The module provides multiple sequencers that provide different
// combinations of encoding speed  and compression ratios. Usually a
// slower sequencer will generate a better compression ratio.
//
// We provide also two decoders. The Decoder slides the decompression
// window through a larger buffer implmented by Buffer. The RingDecoder
// uses the RingBuffer that requires only a slice of the size of the
// window plus 1. The Decoder is significantly faster.
package lz

import "io"

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

// Len returns the complete length of the sequence.
func (s Seq) Len() int64 {
	return int64(s.MatchLen) + int64(s.LitLen)
}

// Block stores sequences and literals. Note that literals that are not consumed
// by the Sequences slice need to be added to the end of the reconstructed data.
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

// Flags for the Sequence function.
const (
	// NoTrailingLiterals tells a sequencer that trailing literals don't
	// need to be included in the block.
	NoTrailingLiterals = 1 << iota
)

// Sequencer transforms byte streams into a block of sequences. The target block
// size under control of the sequencer. The method returns the actual number of
// bytes sequences have been generated for. The block can be reused and will be
// overwritten. If the block is nil k bytes will be skipped and no sequences
// generated.
//
// Sequencer manages an internal buffer that provides a window on the data to be
// compressed.
type Sequencer interface {
	Sequence(blk *Block, flags int) (n int, err error)
}

// WriteSequencer buffers the data to generate LZ77 sequences for. It has
// additional methods required to work with a WrappedSequencer. Requested
// provides the number of bytes that can be written to the WriteSequencer.
//
// The Sequence method will return ErrEmptyBuffer if no data is avaialble in the
// sequencer buffer.
type WriteSequencer interface {
	io.Writer
	WindowSize() int
	Requested() int
	Reset()
	Sequencer
}

// SequencerConfigurator defines a general interface for sequencer
// configurations. The different Sequencers have all different configuration
// parameters and require their own configuration. All configuration types must
// support the NewWriteSequencer method.
//
// Using pattern language that is obviously a factory, but we support multiple
// factories. A configuration structure like HashSequencerConfig creates only
// HashSequencers but the general SequencerConfig structure can build different
// WriteSequencer.
type SequencerConfigurator interface {
	NewWriteSequencer() (s WriteSequencer, err error)
}

// SequencerConfig provides a general method to create sequencers.
type SequencerConfig struct {
	// MemoryBudget specifies the memory budget in bytes for the sequencer. The
	// budget controls how much memory the sequencer has for the window size and the
	// match search data structures. It doesn't control temporary memory
	// allocations. It is a budget, so it can be overdrawn, right?
	MemoryBudget int
	// Effort is scale from 1 to 10 controlling the CPU consumption. A
	// sequencer with an effort of 1 might be extremely fast but will have a
	// worse compression ratio. The default effort is 6 and will provide a
	// reasonable compromise between compression speed and compression
	// ratio. Effort 10 will provide the best compression ratio but will
	// require a higher compression ratio but will be very slow.
	Effort int
	// MaxBlockSize defines a maximum block size. Note that the configurator
	// might create a smaller block size to fit the match search data
	// structures into the memory budget. The main consumer is ZStandard
	// which has a maximum block size of 128 kByte.
	MaxBlockSize int
}

// NewWriteSequencer creates a new sequencer according to the parameters
// provided. The function will only return an error the parameters are negative
// but otherwise always try to satisfy the requirements. If memory size is zero
// the memory budget will be 8 MByte.
func (cfg SequencerConfig) NewWriteSequencer() (s WriteSequencer, err error) {
	panic("TODO")
}
