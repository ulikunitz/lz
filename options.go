package lz

import "fmt"

type ParserType int

const (
	Greedy ParserType = 1 + iota
)

func (pt ParserType) MarshalText() ([]byte, error) {
	switch pt {
	case Greedy:
		return []byte("Greedy"), nil
	default:
		return nil, fmt.Errorf("lz: unknown ParserType %d", pt)
	}
}

func (pt *ParserType) UnmarshalText(text []byte) error {
	switch string(text) {
	case "Greedy":
		*pt = Greedy
		return nil
	default:
		return fmt.Errorf("lz: unknown ParserType %q", text)
	}
}

type MapperType int

const (
	Hash MapperType = 1 + iota
)

func (mt MapperType) MarshalText() ([]byte, error) {
	switch mt {
	case Hash:
		return []byte("Hash"), nil
	default:
		return nil, fmt.Errorf("lz: unknown MapperType %d", mt)
	}
}

func (mt *MapperType) UnmarshalText(text []byte) error {
	switch string(text) {
	case "Hash":
		*mt = Hash
		return nil
	default:
		return fmt.Errorf("lz: unknown MapperType %q", text)
	}
}

type ParserOptions struct {
	// generic options
	BlockSize      int  `json:",omitzero"`
	WindowSize     int  `json:",omitzero"`
	BufferSize     int  `json:",omitzero"`
	NoPruning       bool `json:",omitzero"`
	MaintainWindow bool `json:",omitzero"`
	MinMatchLen    int  `json:",omitzero"`
	MaxMatchLen    int  `json:",omitzero"`

	// specific parser
	// supported parsers: Greedy
	Parser ParserType `json:",omitzero"`

	// specific mapper
	// supported mappers: Hash
	Mapper MapperType `json:",omitzero"`

	// Options for the Hash mapper.
	InputLen int `json:",omitzero"`
	HashBits int `json:",omitzero"`
}

func (opts *ParserOptions) setDefaults() {
	if opts == nil {
		return
	}
	if opts.Parser == 0 {
		opts.Parser = Greedy
	}
	if opts.Mapper == 0 {
		opts.Mapper = Hash
	}
	if opts.BlockSize == 0 {
		opts.BlockSize = 128 << 10
	}
	if opts.WindowSize == 0 {
		opts.WindowSize = 32 << 10
	}
	if opts.BufferSize == 0 {
		opts.BufferSize = 64 << 10
	}
	if opts.MinMatchLen == 0 {
		opts.MinMatchLen = 3
	}
	if opts.MaxMatchLen == 0 {
		opts.MaxMatchLen = 273
	}
	switch opts.Mapper {
	case Hash:
		setHashDefaults(opts)
	}
}

func (opts *ParserOptions) verify() error {
	if opts == nil {
		return fmt.Errorf("lz: parser options are nil")
	}
	if opts.BlockSize <= 0 {
		return fmt.Errorf("lz: invalid BlockSize=%d; must be > 0", opts.BlockSize)
	}
	if opts.WindowSize <= 0 {
		return fmt.Errorf("lz: invalid WindowSize=%d; must be > 0", opts.WindowSize)
	}
	if opts.BufferSize <= 0 {
		return fmt.Errorf("lz: invalid BufferSize=%d; must be > 0", opts.BufferSize)
	}
	if opts.MaintainWindow {
		if opts.NoPruning && opts.BufferSize < opts.WindowSize {
			return fmt.Errorf(
				"lz: invalid options; BufferSize=%d must be >= WindowSize=%d when MaintainWindow is true and NoPurges is true",
				opts.BufferSize, opts.WindowSize)
		}
		if !opts.NoPruning && opts.BufferSize <= opts.WindowSize {
			return fmt.Errorf(
				"lz: invalid options; BufferSize=%d must be > WindowSize=%d when MaintainWindow is true",
				opts.BufferSize, opts.WindowSize)
		}
	}
	if !(2 <= opts.MinMatchLen && opts.MinMatchLen <= opts.MaxMatchLen) {
		return fmt.Errorf("lz: invalid MinMatchLen=%d; must be 3..MaxMatchLen(%d)",
			opts.MinMatchLen, opts.MaxMatchLen)
	}

	switch opts.Parser {
	case Greedy:
		// ok
	default:
		return fmt.Errorf("lz: unknown Parser %d", opts.Parser)

	}
	switch opts.Mapper {
	case Hash:
		if err := verifyHashOptions(opts); err != nil {
			return err
		}
	default:
		return fmt.Errorf("lz: unknown Mapper %d", opts.Mapper)
	}

	return nil
}

func NewParser(opts *ParserOptions) (Parser, error) {
	if opts == nil {
		opts = &ParserOptions{}
	}
	opts.setDefaults()
	if err := opts.verify(); err != nil {
		return nil, err
	}

	switch opts.Parser {
	case Greedy:
		return newGreedyParser(opts)
	default:
		return nil, fmt.Errorf("lz: unknown Parser %d", opts.Parser)
	}
}

func newMatcherOptions(opts *ParserOptions) (matcher, error) {
	var m mapper
	switch opts.Mapper {
	case Hash:
		h := &hash{}
		if err := h.init(opts.InputLen, opts.HashBits); err != nil {
			return nil, err
		}
		m = h
	default:
		return nil, fmt.Errorf("lz: unknown Mapper %d", opts.Mapper)
	}
	return newMatcher(m, opts)
}
