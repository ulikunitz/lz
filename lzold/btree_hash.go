// SPDX-FileCopyrightText: © 2021 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

package lzold

import "fmt"

type bTreeHash struct {
	table    []*bNode
	mask     uint64
	shift    uint
	inputLen int

	workTree bTree
}

func (h *bTreeHash) init(order int, pdata *[]byte, inputLen int, hashBits int) error {
	if err := h.workTree.init(order, pdata); err != nil {
		return err
	}
	if !(2 <= inputLen && inputLen <= 8) {
		return fmt.Errorf("lz: inputLen must be in range [2,8]")
	}
	maxHashBits := 24
	if t := 8 * inputLen; t < maxHashBits {
		maxHashBits = t
	}
	if !(0 <= hashBits && hashBits <= maxHashBits) {
		return fmt.Errorf("lz: hashBits=%d; must be <= %d",
			hashBits, maxHashBits)
	}

	h.table = make([]*bNode, 1<<hashBits)
	h.mask = 1<<(uint(inputLen)*8) - 1
	h.shift = 64 - uint(hashBits)
	h.inputLen = inputLen

	return nil
}

func (h *bTreeHash) Reset(pdata *[]byte) {
	h.workTree.Reset(pdata)
	for i := range h.table {
		h.table[i] = nil
	}
}

func (h *bTreeHash) setMatches(m int) error {
	return h.workTree.setMatches(m)
}

func (h *bTreeHash) Add(pos uint32, x uint64) {
	proot := &h.table[hashValue(x, h.shift)]
	h.workTree.root = *proot
	h.workTree._add(pos)
	*proot = h.workTree.root
}

func (h *bTreeHash) Adapt(delta uint32) {
	for i, root := range h.table {
		h.workTree.root = root
		h.workTree.Adapt(delta)
		h.table[i] = h.workTree.root
	}
}

func (h *bTreeHash) AppendMatchesAndAdd(matches []uint32, pos uint32, x uint64) []uint32 {
	proot := &h.table[hashValue(x, h.shift)]
	h.workTree.root = *proot
	matches = h.workTree.AppendMatchesAndAdd(matches, pos, x)
	*proot = h.workTree.root
	return matches
}

type BTreeHashConfig struct {
	Order    int
	Matches  int
	InputLen int
	HashBits int
}

func (cfg *BTreeHashConfig) Verify() error {
	if cfg.Order < 3 {
		return fmt.Errorf("lz: Order must be >= 3")
	}
	if cfg.Matches < 0 {
		return fmt.Errorf("lz: Matches must be >= 0")
	}
	if !(2 <= cfg.InputLen && cfg.InputLen <= 8) {
		return fmt.Errorf("lz: InputLen must be in range [2,8]")
	}
	maxHashBits := 24
	if t := 8 * cfg.InputLen; t < maxHashBits {
		maxHashBits = t
	}
	if !(0 <= cfg.HashBits && cfg.HashBits <= maxHashBits) {
		return fmt.Errorf("lz: HashBits=%d; must be <= %d",
			cfg.HashBits, maxHashBits)
	}
	return nil
}

func (cfg *BTreeHashConfig) ApplyDefaults() {
	if cfg.Order == 0 {
		cfg.Order = 128
	}
	if cfg.Matches == 0 {
		cfg.Matches = 2
	}
	if cfg.InputLen == 0 {
		cfg.InputLen = 3
	}
	if cfg.HashBits == 0 {
		cfg.HashBits = 18
	}
}

func (cfg *BTreeHashConfig) NewMatchFinder() (mf MatchFinder, err error) {
	cfg.ApplyDefaults()
	if err = cfg.Verify(); err != nil {
		return nil, err
	}
	h := new(bTreeHash)
	if err = h.init(cfg.Order, nil, cfg.InputLen, cfg.HashBits); err != nil {
		return nil, err
	}
	if err = h.setMatches(cfg.Matches); err != nil {
		return nil, err
	}
	return h, nil
}
