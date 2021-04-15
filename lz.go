// Package lz provides encoders and decoders for LZ77 sequences. The term
// sequence used here reflects the usage in the zstd format specification. We
// use the term Sequencer as something that produces Sequences.
//
// TODO: examples
package lz

import "io"

// Seq represents a single Lempel-Ziv 77 Sequence describing a match,
// consisting of the offset, the length of the match and the number of
// literals preceding the match. The Aux field can be used on upper
// layers to provide additional information.
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
	// NoTrailingLiterals tells the sequencer that trailing literals don't
	// need to be included in the block.
	NoTrailingLiterals = 1 << iota
)

// Sequencer transforms byte streams into a block of sequences. The target block size under control of the
// sequencer. The method returns the actual number of bytes sequences have been
// generated for. The block can be reused and will be overwritten. If the block
// is nil k bytes will be skipped and no sequences generated.
type Sequencer interface {
	Sequence(blk *Block, flags int) (n int, err error)
}

// WriteSequencer provide the data to be sequenced using the Writer
// interface. Requested() returns the number of bytes that should be written
// into the sequencer.
type WriteSequencer interface {
	io.Writer
	Requested() int
	Reset()
	Sequencer
}
