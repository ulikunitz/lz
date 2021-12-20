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

// OldSequencer transforms byte streams into a block of sequences. The target block
// size under control of the sequencer. The method returns the actual number of
// bytes sequences have been generated for. The block can be reused and will be
// overwritten. If the block is nil k bytes will be skipped and no sequences
// generated.
//
// OldSequencer manages an internal buffer that provides a window on the data to be
// compressed.
type OldSequencer interface {
	Sequence(blk *Block, flags int) (n int, err error)
}

// InputSequencer buffers the data to generate LZ77 sequences for. It has
// additional methods required to work with a WrappedSequencer. RequestBuffer
// provides the number of bytes that can be written to the InputSequencer.
// ByteAt returns the byte at absolute position pos and returns an error if pos
// refers to a position outside of the current buffer. Pos returns the absolute
// position of the window head.
//
// The Sequence method will return ErrEmptyBuffer if no data is avaialble in the
// sequencer buffer.
type InputSequencer interface {
	OldSequencer
	io.Writer
	io.ReaderFrom
	WindowSize() int
	RequestBuffer() int
	Reset()
	Pos() int64
	ByteAt(pos int64) (c byte, err error)
}

// OldConfigurator defines a general interface for sequencer
// configurations. The different Sequencers have all different configuration
// parameters and require their own configuration. All configuration types must
// support the NewInputSequencer method.
//
// Using pattern language that is obviously a factory, but we support multiple
// factories. A configuration structure like HashSequencerConfig creates only
// HashSequencers but the general SequencerConfig structure can build different
// InputSequencer.
type OldConfigurator interface {
	NewInputSequencer() (s InputSequencer, err error)
}

// Sequencer transforms byte streams into Lempel-Ziv sequences, that allow the
// reconstruction of the input data.
type Sequencer interface {
	// Sequence finds Lempel-Ziv sequences.
	Sequence(blk *Block, blockSize int, flags int) (n int, err error)
	// Shrink reduces the actual window length to make more buffer space
	// available.
	Shrink(newWindowLen int) int
	// WindowPtr returns a pointer to the Window structure.
	WindowPtr() *Window
}

// Configurator generates  new sequencer instances.
type Configurator interface {
	NewSequencer() (s Sequencer, err error)
}
