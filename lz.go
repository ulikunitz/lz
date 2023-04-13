// Package lz provides encoders and decoders for LZ77 sequences. A sequence, as
// described in the zstd specification, describes a number of literal bytes and
// a match.
//
// A Sequencer is an encoder that converts a byte stream into blocks of
// sequences. A Decoder converts the block of sequences into the original
// decompressed byte stream. A wrapped Sequencer reads the byte stream from a
// reader. The sequencers are provided here separately because they are more
// efficient for encoding byte slices directly.
//
// The module provides multiple sequencers that provide different combinations
// of encoding speed  and compression ratios. Usually a slower sequencer will
// generate a better compression ratio.
//
// The [Decoder] slides the decompression window through a larger buffer
// implemented by [DecBuffer].
package lz

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

// Sequencer transforms byte streams into Lempel-Ziv sequences, that allow the
// reconstruction of the input data.
type Sequencer interface {
	// Sequence finds Lempel-Ziv sequences.
	Sequence(blk *Block, flags int) (n int, err error)
	// Shrink reduces the actual window length to make more buffer space
	// available.
	Shrink()
	// Buffer returns a pointer to the sequencer buffer.
	Buffer() *SeqBuffer
	// Reset allows the reuse of the Sequencer. The data slice provides new
	// data to sequence but Sequencers are usually also Writers for
	// providing the data.
	Reset(data []byte) error
}

// SeqConfig generates  new sequencer instances.
type SeqConfig interface {
	NewSequencer() (s Sequencer, err error)
	BufferConfig() *SBConfig
	ApplyDefaults()
	Verify() error
}
