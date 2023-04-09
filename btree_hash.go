package lz

import "fmt"

type bTreeHash struct {
	table    []*bNode
	mask     uint64
	shift    uint
	inputLen int

	workTree bTree
}

func (h *bTreeHash) init(order int, p []byte, inputLen int, hashBits int) error {
	if err := h.workTree.init(order, p); err != nil {
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

func (h *bTreeHash) add(pos uint32, x uint64) {
	proot := &h.table[hashValue(x, h.shift)]
	h.workTree.root = *proot
	h.workTree._add(pos)
	*proot = h.workTree.root
}

func (h *bTreeHash) adapt(delta uint32) {
	for i, root := range h.table {
		h.workTree.root = root
		h.workTree.adapt(delta)
		h.table[i] = h.workTree.root
	}
}

func (h *bTreeHash) appendMatchesAndAdd(matches []uint32, pos uint32, x uint64) []uint32 {
	proot := &h.table[hashValue(x, h.shift)]
	h.workTree.root = *proot
	matches = h.workTree.appendMatchesAndAdd(matches, pos, x)
	*proot = h.workTree.root
	return matches
}
