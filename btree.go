package lz

import (
	"bytes"
	"fmt"
)

// bTree represents a B-tree as described by Donald Knuth. The slice p holds the
// data to compress and we store indexes to that array in the B-tree sorted by
// the suffixes starting at the key positions. Note that we are only supporting
// trees with order 3 or higher.
type bTree struct {
	p     []byte
	root  *bNode
	order int

	// helper field used for debugging
	aux int
}

// m2 returns the ceiling of the order divided by 2.
func (t *bTree) m2() int {
	return (t.order + 1) >> 1
}

// bNode represents a node in the B-tree. We are not storing leaves in the
// tree. In a node that has leaves the length of the children slice will be
// zero.
type bNode struct {
	keys     []uint32
	children []*bNode
}

// init initializes the tree structure
func (t *bTree) init(order int, p []byte) error {
	if order < 3 {
		return fmt.Errorf("lz: order=%d; must be >= %d", order, 3)
	}
	*t = bTree{
		p:     p,
		order: order,
	}
	return nil
}

// newBtree creates a new B-tree. The order must be larger than or equal 3.
func newBtree(order int, p []byte) *bTree {
	t := new(bTree)
	if err := t.init(order, p); err != nil {
		panic(err)
	}
	return t
}

// _add adds a position to a B-Tree by using the bPath type. We assume that the
// pos doesn't exist in the tree already.
func (t *bTree) _add(pos uint32) {
	var p bPath
	p.init(t)
	p._search(pos)
	p._insert(pos)
}

// add adds a position to the binary tree.
func (t *bTree) add(pos uint32, x uint64) {
	t._add(pos)
}

// search searches for a position in the given node.
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

// addMax adds a new position under the assumption that the suffix starting at
// pos is larger than all suffixes added before.
func (t *bTree) addMax(pos uint32) {
	if t.root == nil {
		t.root = &bNode{keys: make([]uint32, 0, t.order-1)}
	}
	up, or := t.addMaxAt(t.root, pos)
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

// addMaxAt adds the a suffix starting at pos to the node under the assumption
// that the suffix is larger than all suffixes stored in the node.
func (t *bTree) addMaxAt(o *bNode, pos uint32) (up uint32, or *bNode) {
	i := len(o.keys)
	if len(o.children) == 0 {
		// We are at he bottom of the tree.
		k := i
		if k+1 < t.order {
			o.keys = o.keys[:k+1]
			o.keys[k] = pos
			return 0, nil
		}
		kr := t.order >> 1
		or = &bNode{keys: make([]uint32, kr, t.order-1)}
		k -= kr
		copy(or.keys, o.keys[k:])
		o.keys = o.keys[:k]
		i -= k + 1
		up = or.keys[0]
		copy(or.keys[:i], or.keys[1:])
		or.keys[i] = pos
		return up, or
	}
	// Care for the children!
	var ot *bNode
	pos, ot = t.addMaxAt(o.children[i], pos)
	if ot == nil {
		return 0, nil
	}
	k := len(o.keys)
	if k+1 < t.order {
		o.keys = o.keys[:k+1]
		o.keys[i] = pos
		o.children = o.children[:len(o.children)+1]
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
	i -= k + 1
	up = or.keys[0]
	copy(or.keys[:i], or.keys[1:])
	or.keys[i] = pos
	copy(or.children[:i+1], or.children[1:])
	or.children[i+1] = ot
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

// delMax deletes the largest suffix from the node and returns its position r if
// it can be found.
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

// del deletes the suffix starting at pos from the node.
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

// delete removes the suffix starting at position pos from the B-tree.
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

// walkNode calls function f in sequence of the sorted keys in the subtree
// starting at o.
func (t *bTree) walkNode(o *bNode, f func([]uint32)) {
	if o == nil {
		return
	}
	if len(o.children) == 0 {
		f(o.keys)
		return
	}
	for i := range o.keys {
		t.walkNode(o.children[i], f)
		f(o.keys[i : i+1])
	}
	t.walkNode(o.children[len(o.children)-1], f)
}

// walks calls f for the key in the B-tree in their sorted order.
func (t *bTree) walk(f func(p []uint32)) {
	t.walkNode(t.root, f)
}

// adapt moves the content of the byte slices s bytes to the left and modifies
// the B-tree accordingly. The current implementation recreates the B-tree. Note
// that the shift in the slice must have been done, before calling adapt.
func (t *bTree) adapt(s uint32) {
	u := &bTree{order: t.order, p: t.p}
	f := func(p []uint32) {
		for _, k := range p {
			if k < s {
				continue
			}
			u.addMax(k - s)
		}
	}
	t.walk(f)
	t.root = u.root
}

func (t *bTree) appendMatchesAndAdd(matches []uint32, pos uint32, x uint64) []uint32 {
	panic("TODO")
}

// bEdge describes an edge in the b-Tree. The field node must not be nil it must
// be 0 <= i <= len(o.keys), if len(o.children) > 0 then 0 <= i <
// len(o.children).
type bEdge struct {
	o *bNode
	i int
}

// bPath describes a path from the root to a node in the B-Tree allowing for
// leaf nodes as in the definition by Donald Knuth. We are storing a pointer to
// the tree to allow read-only operations like next and prev but also write
// operations on the tree using the path.
//
// The stack s describes the path from the root through the b-Tree.
//
// Ab empty path with len(p.s) == 0 plays a special role for next and prev to find the first or
// last element of the tree.
type bPath struct {
	s []bEdge
	t *bTree
}

// init initializes the path to be empty.
func (p *bPath) init(t *bTree) {
	if p.s == nil {
		p.s = make([]bEdge, 0, 8)
	} else {
		p.s = p.s[:0]
	}
	p.t = t
}

// reset resets the path to empty
func (p *bPath) reset() {
	p.s = p.s[:0]
}

func (p *bPath) isEmpty() bool { return len(p.s) == 0 }

func (p *bPath) subtree() *bNode {
	if len(p.s) == 0 {
		return p.t.root
	}
	e := &p.s[len(p.s)-1]
	o := e.o
	if len(o.children) == 0 {
		return nil
	}
	return o.children[e.i]
}

// _search assumes that pos is not already part of the tree and should be faster
// in that case.
func (p *bPath) _search(pos uint32) {
	o := p.subtree()
	if o == nil {
		return
	}
	search := p.t.search
	for {
		i := search(o, pos)
		p.append(o, i)
		if len(o.children) == 0 {
			return
		}
		o = o.children[i]
	}
}

func (p *bPath) search(pos uint32) {
	o := p.subtree()
	if o == nil {
		return
	}
	search := p.t.search
	for {
		i := search(o, pos)
		p.append(o, i)
		if len(o.children) == 0 {
			return
		}
		if i < len(o.keys) && o.keys[i] == pos {
			return
		}
		o = o.children[i]
	}
}

func (p *bPath) clone() *bPath {
	s := make([]bEdge, len(p.s), cap(p.s))
	copy(s, p.s)
	return &bPath{s: s, t: p.t}
}

func (p *bPath) append(o *bNode, i int) {
	p.s = append(p.s, bEdge{o: o, i: i})
}

func (p *bPath) max() {
	o := p.subtree()
	if o == nil {
		return
	}
	for len(o.children) > 0 {
		i := len(o.children) - 1
		p.append(o, i)
		o = o.children[i]
	}
	i := len(o.keys) - 1
	p.append(o, i)
}

func (p *bPath) min() {
	o := p.subtree()
	if o == nil {
		return
	}
	for {
		p.append(o, 0)
		if len(o.children) == 0 {
			return
		}
		o = o.children[0]
	}
}

func (p *bPath) next() {
	j := len(p.s) - 1
	if j < 0 {
		p.min()
		return
	}
	e := &p.s[j]
	e.i++
	if e.i < len(e.o.children) {
		p.min()
		return
	}
	for {
		if e.i < len(e.o.keys) {
			return
		}
		p.s = p.s[:j]
		j--
		if j < 0 {
			return
		}
		e = &p.s[j]
	}
}

func (p *bPath) prev() {
	j := len(p.s) - 1
	if j < 0 {
		p.max()
		return
	}
	e := &p.s[j]
	if len(e.o.children) > 0 {
		p.max()
		return
	}
	for {
		e.i--
		if e.i >= 0 {
			return
		}
		p.s = p.s[:j]
		j--
		if j < 0 {
			return
		}
		e = &p.s[j]
	}

}

func (p *bPath) key() (pos uint32, ok bool) {
	j := len(p.s) - 1
	if j < 0 {
		return 0, false
	}
	e := p.s[j]
	o, i := e.o, e.i
	if i >= len(o.keys) {
		return 0, false
	}
	return o.keys[i], true
}

// _insert adds pos32 before the position that p points to. Note that the path
// must always point to a node with leaves (len(o.children) == 0). The path
// is undefined after the operation.
func (p *bPath) _insert(pos uint32) {
	t := p.t
	j := len(p.s) - 1
	if j < 0 {
		t.root = &bNode{keys: make([]uint32, 1, t.order-1)}
		t.root.keys[0] = pos
		return
	}
	e := &p.s[j]
	o, i := e.o, e.i
	if len(o.children) > 0 {
		panic(fmt.Errorf("len(o.children) is %d; want zero",
			len(o.children)))
	}
	k := len(o.keys)
	if k+1 < t.order {
		o.keys = o.keys[:k+1]
		copy(o.keys[i+1:], o.keys[i:])
		o.keys[i] = pos
		return
	}

	kr := t.order >> 1
	or := &bNode{keys: make([]uint32, kr, t.order-1)}
	k -= kr
	copy(or.keys, o.keys[k:])
	o.keys = o.keys[:k]
	var up uint32
	switch {
	case i > k:
		i -= k+1
		up = or.keys[0]
		copy(or.keys[:i], or.keys[1:])
		or.keys[i] = pos
	case i < k:
		up = o.keys[k-1]
		copy(o.keys[i+1:], o.keys[i:])
		o.keys[i] = pos
	default: // i == k
		up = pos
	}
	for {
		j--
		if j < 0 {
			root := &bNode{
				keys:     make([]uint32, 1, t.order-1),
				children: make([]*bNode, 2, t.order),
			}
			root.keys[0] = up
			root.children[0] = t.root
			root.children[1] = or
			t.root = root
			return
		}
		e := &p.s[j]
		o, i = e.o, e.i
		k = len(o.keys)
		if k+1 < t.order {
			o.keys = o.keys[:k+1]
			copy(o.keys[i+1:], o.keys[i:])
			o.keys[i] = up
			o.children = o.children[:len(o.children)+1]
			copy(o.children[i+2:], o.children[i+1:])
			o.children[i+1] = or
			return
		}
		pos = up
		kr = t.order >> 1
		ot := or
		or = &bNode{
			keys: make([]uint32, kr, t.order-1),
			children: make([]*bNode, kr+1, t.order),
		}
		k -= kr
		copy(or.keys, o.keys[k:])
		o.keys = o.keys[:k]
		copy(or.children, o.children[k:])
		o.children = o.children[:k+1]
		switch {
		case i > k:
			i -= k+1
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
		default: // i == k
			or.children[0] = ot
		}
	}
}
