package lz

import (
	"bytes"
	"fmt"
)

// bTree represents a B-Tree as described by Donald Knuth. The slice p holds
// the data to compress and we store indexes to that array in the B-Tree sorted
// by the bytes slices p[key:]. Note that we are only supporting trees with
// order 3 or higher.
type bTree struct {
	p     []byte
	root  *bNode
	order int

	// helper field used for debugging
	aux int
}

func (t *bTree) m2() int {
	return (t.order + 1) >> 1
}

type bNode struct {
	keys     []uint32
	children []*bNode
}

func newBtree(order int, p []byte) *bTree {
	if order < 3 {
		panic(fmt.Errorf("lz: order=%d; must be >= %d", order, 3))
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
		kr := t.order >> 1
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
	kr := t.order >> 1
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

func (t *bTree) stealRight(o *bNode, i int) bool {
	if i >= len(o.keys) {
		return false
	}
	or := o.children[i+1]
	if len(or.keys) < t.m2() {
		return false
	}
	ol := o.children[i]
	k := len(ol.keys)
	ol.keys = ol.keys[:k+1]
	ol.keys[k], o.keys[i] = o.keys[i], or.keys[0]
	copy(or.keys, or.keys[1:])
	or.keys = or.keys[:len(or.keys)-1]
	if len(ol.children) == 0 {
		return true
	}
	k++
	ol.children = ol.children[:k+1]
	ol.children[k] = or.children[0]
	copy(or.children, or.children[1:])
	or.children = or.children[:len(or.children)-1]
	return true
}

func (t *bTree) stealLeft(o *bNode, i int) bool {
	if i <= 0 {
		return false
	}
	i--
	ol := o.children[i]
	if len(ol.keys) < t.m2() {
		return false
	}
	or := o.children[i+1]
	or.keys = or.keys[:len(or.keys)+1]
	copy(or.keys[1:], or.keys)
	k := len(ol.keys) - 1
	or.keys[0], o.keys[i] = o.keys[i], ol.keys[k]
	ol.keys = ol.keys[:k]
	if len(ol.children) == 0 {
		return true
	}
	k++
	or.children = or.children[:len(or.children)+1]
	copy(or.children[1:], or.children)
	or.children[0] = ol.children[k]
	ol.children = ol.children[:k]
	return true
}

func (t *bTree) dropKey(o *bNode, i int) {
	ol, or := o.children[i], o.children[i+1]
	k := len(ol.keys)
	ol.keys = ol.keys[:k+1+len(or.keys)]
	ol.keys[k] = o.keys[i]
	copy(ol.keys[k+1:], or.keys)
	copy(o.keys[i:], o.keys[i+1:])
	o.keys = o.keys[:len(o.keys)-1]
	i++
	copy(o.children[i:], o.children[i+1:])
	o.children = o.children[:len(o.children)-1]
	if len(ol.children) == 0 {
		return
	}
	k = len(ol.children)
	ol.children = ol.children[:k+len(or.children)]
	copy(ol.children[k:], or.children)
}

func (t *bTree) delMax(o *bNode) (r uint32, ok bool) {
	i := len(o.keys)
	if len(o.children) == 0 {
		if i == 0 {
			return 0, false
		}
		i--
		r = o.keys[i]
		o.keys = o.keys[:i]
		return r, true
	}
	oc := o.children[i]
	r, ok = t.delMax(oc)
	if !ok {
		panic("unexpected; children should have more than m/2 entries")
	}
	if len(oc.keys)+1 >= t.m2() {
		return r, true
	}
	if t.stealLeft(o, i) {
		return r, true
	}
	t.dropKey(o, i-1)
	return r, true
}

func (t *bTree) del(o *bNode, pos uint32) {
	i := t.search(o, pos)
	if len(o.children) == 0 {
		if i >= len(o.keys) || o.keys[i] != pos {
			return
		}
		copy(o.keys[i:], o.keys[i+1:])
		o.keys = o.keys[:len(o.keys)-1]
		return
	}
	oc := o.children[i]
	if i < len(o.keys) && o.keys[i] == pos {
		var ok bool
		o.keys[i], ok = t.delMax(oc)
		if !ok {
			panic("unexpected")
		}
	} else {
		t.del(oc, pos)
	}
	if len(oc.keys)+1 >= t.m2() {
		return
	}
	if t.stealRight(o, i) {
		return
	}
	if t.stealLeft(o, i) {
		return
	}
	if i == len(o.keys) {
		i--
	}
	t.dropKey(o, i)
}

// delete removes position pos from the B-Tree.
func (t *bTree) delete(pos uint32) {
	if t.root == nil {
		return
	}
	t.del(t.root, pos)
	switch len(t.root.children) {
	case 0:
		if len(t.root.keys) == 0 {
			t.root = nil
		}
	case 1:
		t.root = t.root.children[0]
	}
}
