// SPDX-FileCopyrightText: © 2021 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

package lzold

import (
	"fmt"
)

// TODO: Only calculate window size and hash sizes for the configurator.

// Params provides a general method to create sequencers.
type Params struct {
	// MemoryBudget specifies the memory budget in bytes for the sequencer. The
	// budget controls how much memory the sequencer has for the window size and the
	// match search data structures. It doesn't control temporary memory
	// allocations. It is a budget, so it can be overdrawn, right?
	MemoryBudget int
	// Effort is scale from 1 to 10 controlling the CPU consumption. A
	// sequencer with an effort of 1 might be extremely fast but will have a
	// worse compression ratio. The default effort is 6 and will provide a
	// reasonable compromise between compression speed and compression
	// ratio. Effort 10 will provide the best compression ratio but will be
	// very slow.
	Effort int
	// BlockSize defines a maximum block size. Note that the configurator
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
func (p *Params) ApplyDefaults() {
	if p.Effort == 0 {
		p.Effort = 5
	}
	if p.BlockSize == 0 {
		p.BlockSize = 128 * 1024
	}
	// WindowSize and MemoryBudget stays 0 if none is given
}

// Verify checks the configuration for errors. Use ApplyDefaults before this
// function because it doesn't support zero values in all cases.
func (p *Params) Verify() error {
	if p.MemoryBudget < 0 {
		return fmt.Errorf("lz: cfg.MemoryBudget must be positive")
	}
	if !(1 <= p.Effort && p.Effort <= 9) {
		return fmt.Errorf("lz: cfg.Effort must be in range 1-9")
	}
	if p.BlockSize <= 0 {
		return fmt.Errorf("lz: BlockSize must be positive")
	}
	if p.WindowSize < 0 {
		return fmt.Errorf("lz: cfg.WindowSize must be non-negative")
	}
	if p.MemoryBudget > 0 && p.WindowSize > p.MemoryBudget {
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
func computeConfig(cfg Params) (c SeqConfig, err error) {
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
		hscfg := HSConfig{
			InputLen: hsParams.inputLen,
			HashBits: hsParams.bits,
		}
		hscfg.WindowSize = cfg.MemoryBudget - (1<<hscfg.HashBits)*8 -
			161 + hscfg.InputLen
		hscfg.ApplyDefaults()
		return &hscfg, nil
	case 3, 4:
		hsParams := findHSParams(bhsParameters, cfg.MemoryBudget,
			memSizeHS)
		bhscfg := BHSConfig{
			InputLen: hsParams.inputLen,
			HashBits: hsParams.bits,
		}
		bhscfg.WindowSize = cfg.MemoryBudget - (1<<bhscfg.HashBits)*8 -
			161 + bhscfg.InputLen
		bhscfg.ApplyDefaults()
		return &bhscfg, nil
	case 5, 6, 7:
		dhsParams := findDHSParams(dhsParameters, cfg.MemoryBudget,
			memSizeDHS)
		dhscfg := DHSConfig{
			InputLen1: dhsParams.inputLen1,
			HashBits1: dhsParams.bits1,
			InputLen2: dhsParams.inputLen2,
			HashBits2: dhsParams.bits2,
		}
		dhscfg.WindowSize = cfg.MemoryBudget - 207 -
			(1<<dhscfg.HashBits1)*8 -
			(1<<dhscfg.HashBits2)*8
		dhscfg.ApplyDefaults()
		return &dhscfg, nil
	case 8, 9:
		bdhsParams := findDHSParams(bdhsParameters, cfg.MemoryBudget,
			memSizeDHS)
		bdhscfg := BDHSConfig{
			InputLen1: bdhsParams.inputLen1,
			HashBits1: bdhsParams.bits1,
			InputLen2: bdhsParams.inputLen2,
			HashBits2: bdhsParams.bits2,
		}
		bdhscfg.WindowSize = cfg.MemoryBudget - 207 -
			(1<<bdhscfg.HashBits1)*8 -
			(1<<bdhscfg.HashBits2)*8
		bdhscfg.ApplyDefaults()
		return &bdhscfg, nil
	default:
		panic("unreachable")
	}
}

// computeConfigWindow computes the configuration for a given window size.
func computeConfigWindow(params Params) (c SeqConfig, err error) {
	if !(1 <= params.Effort && params.Effort <= 9) {
		return nil, fmt.Errorf("lz: effort %d not supported",
			params.Effort)
	}
	if params.MemoryBudget == 0 {
		params.MemoryBudget = params.WindowSize +
			memoryBudgetTable[params.Effort]
	}
	b := params.WindowSize + 1024
	if b > params.MemoryBudget {
		params.MemoryBudget = b
	}
	switch params.Effort {
	case 1, 2:
		p := windowHS(hsWinParameters, params.WindowSize)
		hsParams := findHSParams(p, params.MemoryBudget, memSizeHSWin)
		hscfg := HSConfig{
			InputLen: hsParams.inputLen,
			HashBits: hsParams.bits,
		}
		hscfg.WindowSize = params.WindowSize
		hscfg.ApplyDefaults()
		return &hscfg, nil
	case 3, 4:
		p := windowHS(bhsWinParameters, params.WindowSize)
		hsParams := findHSParams(p, params.MemoryBudget, memSizeHSWin)
		bhscfg := BHSConfig{
			InputLen: hsParams.inputLen,
			HashBits: hsParams.bits,
		}
		bhscfg.WindowSize = params.WindowSize
		bhscfg.ApplyDefaults()
		return &bhscfg, nil
	case 5, 6, 7:
		p := windowDHS(dhsWinParameters, params.WindowSize)
		dhsParams := findDHSParams(p, params.MemoryBudget,
			memSizeDHSWin)
		dhscfg := DHSConfig{
			InputLen1: dhsParams.inputLen1,
			HashBits1: dhsParams.bits1,
			InputLen2: dhsParams.inputLen2,
			HashBits2: dhsParams.bits2,
		}
		dhscfg.WindowSize = params.WindowSize
		dhscfg.ApplyDefaults()
		return &dhscfg, nil
	case 8, 9:
		p := windowDHS(bdhsWinParameters, params.WindowSize)
		bdhsParams := findDHSParams(p, params.MemoryBudget,
			memSizeDHSWin)
		bdhscfg := BDHSConfig{
			InputLen1: bdhsParams.inputLen1,
			HashBits1: bdhsParams.bits1,
			InputLen2: bdhsParams.inputLen2,
			HashBits2: bdhsParams.bits2,
		}
		bdhscfg.WindowSize = params.WindowSize
		bdhscfg.ApplyDefaults()
		return &bdhscfg, nil
	default:
		panic("unreachable")
	}
}

// Config converts the parameters into an actual configuration.
func Config(p Params) (c SeqConfig, err error) {
	p.ApplyDefaults()
	if err = p.Verify(); err != nil {
		return nil, err
	}
	if p.WindowSize == 0 {
		return computeConfig(p)
	}
	return computeConfigWindow(p)

}

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
