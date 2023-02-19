package lz

import (
	"bytes"
	"fmt"
)

// bTree represents a B-Tree as described by Donald Knuth. The slice p holds
// the data to compress and we store indexes to that array in the B-Tree sorted
// by the bytes slices p[key:].
type bTree struct {
	p     []byte
	root  *bNode
	order int
	// depth?
}

type bNode struct {
	keys     []uint32
	children []*bNode
}

func newBtree(order int, p []byte) *bTree {
	if order < 2 {
		panic(fmt.Errorf("lz: order=%d; must be >= %d", order, 2))
	}
	return &bTree{
		p:     p,
		root:  nil,
		order: order,
	}
}

func (t *bTree) add(pos uint32) {
	if t.root == nil {
		t.root = &bNode{keys: make([]uint32, 0, t.order-1)}
	}
	up, or := t.addAt(t.root, pos)
	if or == nil {
		return
	}
	root := &bNode{
		keys:     make([]uint32, 1, t.order-1),
		children: make([]*bNode, 2, t.order),
	}
	root.keys[0] = up
	root.children[0] = t.root
	root.children[1] = or
	t.root = root
}

func (t *bTree) search(o *bNode, pos uint32) int {
	q := t.p[pos:]
	p := t.p
	keys := o.keys
	i, j := 0, len(keys)
	for i < j {
		h := int(uint(i+j) >> 1)
		if bytes.Compare(q, p[keys[h]:]) > 0 {
			i = h + 1
		} else {
			j = h
		}
	}
	return i
}

func (t *bTree) addAt(o *bNode, pos uint32) (up uint32, or *bNode) {
	i := t.search(o, pos)
	if len(o.children) == 0 {
		// We are at he bottom of the tree.
		k := len(o.keys)
		if k+1 < t.order {
			o.keys = o.keys[:k+1]
			copy(o.keys[i+1:], o.keys[i:])
			o.keys[i] = pos
			return 0, nil
		}
		kr := (t.order >> 1) - 1
		or = &bNode{keys: make([]uint32, kr, t.order-1)}
		k -= kr
		copy(or.keys, o.keys[k:])
		o.keys = o.keys[:k]
		switch {
		case i == k:
			up = pos
		case i > k:
			i -= k + 1
			up = or.keys[0]
			copy(or.keys[:i], or.keys[1:])
			or.keys[i] = pos
		case i < k:
			up = o.keys[k-1]
			copy(o.keys[i+1:], o.keys[i:])
			o.keys[i] = pos
		}
		return up, or
	}
	// Care for the children!
	var ot *bNode
	pos, ot = t.addAt(o.children[i], pos)
	if ot == nil {
		return 0, nil
	}
	k := len(o.keys)
	if k+1 < t.order {
		o.keys = o.keys[:k+1]
		copy(o.keys[i+1:], o.keys[i:])
		o.keys[i] = pos
		o.children = o.children[:len(o.children)+1]
		copy(o.children[i+2:], o.children[i+1:])
		o.children[i+1] = ot
		return 0, nil
	}
	kr := (t.order >> 1) - 1
	or = &bNode{
		keys:     make([]uint32, kr, t.order-1),
		children: make([]*bNode, kr+1, t.order),
	}
	k -= kr
	copy(or.keys, o.keys[k:])
	o.keys = o.keys[:k]
	copy(or.children, o.children[k:])
	o.children = o.children[:k+1]
	switch {
	case i == k:
		up = pos
		or.children[0] = ot
	case i > k:
		i -= k + 1
		up = or.keys[0]
		copy(or.keys[:i], or.keys[1:])
		or.keys[i] = pos
		copy(or.children[:i+1], or.children[1:])
		or.children[i+1] = ot
	case i < k:
		up = o.keys[k-1]
		copy(o.keys[i+1:], o.keys[i:])
		o.keys[i] = pos
		copy(o.children[i+2:], o.children[i+1:])
		o.children[i+1] = ot
	}
	return up, or

}
