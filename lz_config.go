package lz

import (
	"fmt"
)

// Config provides a general method to create sequencers.
type Config struct {
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
	BlockSize int
	// WindowSize fixes the window size.
	WindowSize int
}

// ApplyDefaults applies the defaults to the Config structure. The memory budget
// is set to 2 MB, the effort to 5 and the block size to 128 kByte unless no
// other non-zero values have been set.
func (cfg *Config) ApplyDefaults() {
	if cfg.Effort == 0 {
		cfg.Effort = 5
	}
	if cfg.BlockSize == 0 {
		cfg.BlockSize = 128 * 1024
	}
	// WindowSize and MemoryBudget stays 0 if none is given
}

// Verify checks the configuration for errors. Use ApplyDefaults before this
// function because it doesn't support zero values in all cases.
func (cfg *Config) Verify() error {
	if cfg.MemoryBudget < 0 {
		return fmt.Errorf("lz: cfg.MemoryBudget must be positive")
	}
	if !(1 <= cfg.Effort && cfg.Effort <= 9) {
		return fmt.Errorf("lz: cfg.Effort must be in range 1-9")
	}
	if cfg.BlockSize <= 0 {
		return fmt.Errorf("lz: BlockSize must be positive")
	}
	if cfg.WindowSize < 0 {
		return fmt.Errorf("lz: cfg.WindowSize must be non-negative")
	}
	if cfg.MemoryBudget > 0 && cfg.WindowSize > cfg.MemoryBudget {
		return fmt.Errorf("lz: memory budget must be larger" +
			" or equal window size")
	}
	return nil
}

var memoryBudgetTable = []int{
	1: 768 * kb,
	2: 2 * mb,
	3: 768 * kb,
	4: 2 * mb,
	5: 768 * kb,
	6: 2 * mb,
	7: 8 * mb,
	8: 768 * kb,
	9: 8 * mb,
}

// computeConfig computes the configuration extremely fast.
func computeConfig(cfg Config) (c OldConfigurator, err error) {
	if !(1 <= cfg.Effort && cfg.Effort <= 9) {
		return nil, fmt.Errorf("lz: effort %d not supported",
			cfg.Effort)
	}
	if cfg.MemoryBudget == 0 {
		cfg.MemoryBudget = memoryBudgetTable[cfg.Effort]
	}
	switch cfg.Effort {
	case 1, 2:
		hsParams := findHSParams(hsParameters, cfg.MemoryBudget,
			memSizeHS)
		hscfg := OHSConfig{
			BlockSize: cfg.BlockSize,
			InputLen:  hsParams.inputLen,
			HashBits:  hsParams.bits,
		}
		hscfg.WindowSize = cfg.MemoryBudget - (1<<hscfg.HashBits)*8 -
			161 + hscfg.InputLen
		hscfg.MaxSize = hscfg.WindowSize
		if hscfg.WindowSize < 64*kb {
			hscfg.ShrinkSize = hscfg.WindowSize / 2
		} else {
			hscfg.ShrinkSize = 32 * kb
		}
		return &hscfg, nil
	case 3, 4:
		hsParams := findHSParams(bhsParameters, cfg.MemoryBudget,
			memSizeHS)
		bhscfg := OBHSConfig{
			BlockSize: cfg.BlockSize,
			InputLen:  hsParams.inputLen,
			HashBits:  hsParams.bits,
		}
		bhscfg.WindowSize = cfg.MemoryBudget - (1<<bhscfg.HashBits)*8 -
			161 + bhscfg.InputLen
		bhscfg.MaxSize = bhscfg.WindowSize
		if bhscfg.WindowSize < 64*kb {
			bhscfg.ShrinkSize = bhscfg.WindowSize / 2
		} else {
			bhscfg.ShrinkSize = 32 * kb
		}
		return &bhscfg, nil
	case 5, 6, 7:
		dhsParams := findDHSParams(dhsParameters, cfg.MemoryBudget,
			memSizeDHS)
		dhscfg := ODHSConfig{
			BlockSize: cfg.BlockSize,
			InputLen1: dhsParams.inputLen1,
			HashBits1: dhsParams.bits1,
			InputLen2: dhsParams.inputLen2,
			HashBits2: dhsParams.bits2,
		}
		dhscfg.WindowSize = cfg.MemoryBudget - 207 -
			(1<<dhscfg.HashBits1)*8 -
			(1<<dhscfg.HashBits2)*8
		dhscfg.MaxSize = dhscfg.WindowSize
		if dhscfg.WindowSize < 64*kb {
			dhscfg.ShrinkSize = dhscfg.WindowSize / 2
		} else {
			dhscfg.ShrinkSize = 32 * kb
		}
		return &dhscfg, nil
	case 8, 9:
		bdhsParams := findDHSParams(bdhsParameters, cfg.MemoryBudget,
			memSizeDHS)
		bdhscfg := BDHSConfig{
			BlockSize: cfg.BlockSize,
			InputLen1: bdhsParams.inputLen1,
			HashBits1: bdhsParams.bits1,
			InputLen2: bdhsParams.inputLen2,
			HashBits2: bdhsParams.bits2,
		}
		bdhscfg.WindowSize = cfg.MemoryBudget - 207 -
			(1<<bdhscfg.HashBits1)*8 -
			(1<<bdhscfg.HashBits2)*8
		bdhscfg.MaxSize = bdhscfg.WindowSize
		if bdhscfg.WindowSize < 64*kb {
			bdhscfg.ShrinkSize = bdhscfg.WindowSize / 2
		} else {
			bdhscfg.ShrinkSize = 32 * kb
		}
		return &bdhscfg, nil
	default:
		panic("unreachable")
	}
}

// computeConfigWindow computes the configuration for a given window size.
func computeConfigWindow(cfg Config) (c OldConfigurator, err error) {
	if !(1 <= cfg.Effort && cfg.Effort <= 9) {
		return nil, fmt.Errorf("lz: effort %d not supported",
			cfg.Effort)
	}
	if cfg.MemoryBudget == 0 {
		cfg.MemoryBudget = cfg.WindowSize +
			memoryBudgetTable[cfg.Effort]
	}
	b := cfg.WindowSize + 1024
	if b > cfg.MemoryBudget {
		cfg.MemoryBudget = b
	}
	switch cfg.Effort {
	case 1, 2:
		p := windowHS(hsWinParameters, cfg.WindowSize)
		hsParams := findHSParams(p, cfg.MemoryBudget, memSizeHSWin)
		hscfg := OHSConfig{
			BlockSize: cfg.BlockSize,
			InputLen:  hsParams.inputLen,
			HashBits:  hsParams.bits,
		}
		hscfg.WindowSize = cfg.WindowSize
		hscfg.MaxSize = hscfg.WindowSize
		if hscfg.WindowSize < 64*kb {
			hscfg.ShrinkSize = hscfg.WindowSize / 2
		} else {
			hscfg.ShrinkSize = 32 * kb
		}
		return &hscfg, nil
	case 3, 4:
		p := windowHS(bhsWinParameters, cfg.WindowSize)
		hsParams := findHSParams(p, cfg.MemoryBudget, memSizeHSWin)
		bhscfg := OBHSConfig{
			BlockSize: cfg.BlockSize,
			InputLen:  hsParams.inputLen,
			HashBits:  hsParams.bits,
		}
		bhscfg.WindowSize = cfg.WindowSize
		bhscfg.MaxSize = bhscfg.WindowSize
		if bhscfg.WindowSize < 64*kb {
			bhscfg.ShrinkSize = bhscfg.WindowSize / 2
		} else {
			bhscfg.ShrinkSize = 32 * kb
		}
		return &bhscfg, nil
	case 5, 6, 7:
		p := windowDHS(dhsWinParameters, cfg.WindowSize)
		dhsParams := findDHSParams(p, cfg.MemoryBudget, memSizeDHSWin)
		dhscfg := ODHSConfig{
			BlockSize: cfg.BlockSize,
			InputLen1: dhsParams.inputLen1,
			HashBits1: dhsParams.bits1,
			InputLen2: dhsParams.inputLen2,
			HashBits2: dhsParams.bits2,
		}
		dhscfg.WindowSize = cfg.WindowSize
		dhscfg.MaxSize = dhscfg.WindowSize
		if dhscfg.WindowSize < 64*kb {
			dhscfg.ShrinkSize = dhscfg.WindowSize / 2
		} else {
			dhscfg.ShrinkSize = 32 * kb
		}
		return &dhscfg, nil
	case 8, 9:
		p := windowDHS(bdhsWinParameters, cfg.WindowSize)
		bdhsParams := findDHSParams(p, cfg.MemoryBudget, memSizeDHSWin)
		bdhscfg := BDHSConfig{
			BlockSize: cfg.BlockSize,
			InputLen1: bdhsParams.inputLen1,
			HashBits1: bdhsParams.bits1,
			InputLen2: bdhsParams.inputLen2,
			HashBits2: bdhsParams.bits2,
		}
		bdhscfg.WindowSize = cfg.WindowSize
		bdhscfg.MaxSize = bdhscfg.WindowSize
		if bdhscfg.WindowSize < 64*kb {
			bdhscfg.ShrinkSize = bdhscfg.WindowSize / 2
		} else {
			bdhscfg.ShrinkSize = 32 * kb
		}
		return &bdhscfg, nil
	default:
		panic("unreachable")
	}
}

func (cfg Config) computeConfig() (c OldConfigurator, err error) {
	cfg.ApplyDefaults()
	if err = cfg.Verify(); err != nil {
		return nil, err
	}
	if cfg.WindowSize == 0 {
		return computeConfig(cfg)
	}
	return computeConfigWindow(cfg)

}

// NewInputSequencer creates a new sequencer according to the parameters
// provided. The function will only return an error the parameters are negative
// but otherwise always try to satisfy the requirements.
func (cfg *Config) NewInputSequencer() (s InputSequencer, err error) {
	c, err := cfg.computeConfig()
	if err != nil {
		return nil, err
	}
	return c.NewInputSequencer()
}

const (
	kb = 1024
	mb = 1024 * 1024
)

type hsParams struct {
	size     int
	inputLen int
	bits     int
}

type dhsParams struct {
	size      int
	inputLen1 int
	bits1     int
	inputLen2 int
	bits2     int
}

func memSizeHS(p hsParams) int { return p.size }

func memSizeHSWin(p hsParams) int {
	return (1<<p.bits)*8 + 161 - p.inputLen + p.size
}

func findHSParams(p []hsParams, m int, mem func(hsParams) int) hsParams {
	a := 0
	b := len(p) - 1
	for a < b {
		i := (a + b + 1) / 2
		if m < mem(p[i]) {
			b = i - 1
			continue
		}
		a = i
	}
	return p[a]
}

func memSizeDHS(p dhsParams) int { return p.size }

func memSizeDHSWin(p dhsParams) int {
	return ((1<<p.bits1)+(1<<p.bits2))*8 + 207 + p.size
}

func findDHSParams(p []dhsParams, m int, mem func(dhsParams) int) dhsParams {
	a := 0
	b := len(p) - 1
	for a < b {
		i := (a + b + 1) / 2
		if m < mem(p[i]) {
			b = i - 1
			continue
		}
		a = i
	}
	return p[a]
}

func windowHS(p []hsParams, winSize int) []hsParams {
	if len(p) == 0 {
		return nil
	}
	if winSize < p[0].size {
		winSize = p[0].size
	}
	a, b := 0, len(p)
	for a < b {
		i := (a + b) / 2
		s := p[i].size
		if winSize < s {
			b = i
			continue
		}
		a = i + 1
	}
	v := a
	a, b = 0, v-1
	winSize = p[b].size
	for a < b {
		i := (a + b) / 2
		s := p[i].size
		if s < winSize {
			a = i + 1
			continue
		}
		b = i
	}
	return p[a:v]
}

func windowDHS(p []dhsParams, winSize int) []dhsParams {
	if len(p) == 0 {
		return nil
	}
	if winSize < p[0].size {
		winSize = p[0].size
	}
	a, b := 0, len(p)
	for a < b {
		i := (a + b) / 2
		s := p[i].size
		if winSize < s {
			b = i
			continue
		}
		a = i + 1
	}
	v := a
	a, b = 0, v-1
	winSize = p[b].size
	for a < b {
		i := (a + b) / 2
		s := p[i].size
		if s < winSize {
			a = i + 1
			continue
		}
		b = i
	}
	return p[a:v]
}
