package lz

import (
	"bytes"
	"fmt"
)

// btMask is the mask for the little endian 64-bit representation of the input
// value. We use 3 characters as input for the hash.
const btMask = 1<<24 - 1

// bTrees stores B-Trees in a hash table. The B-Trees follow the definition of
// Donald Knuth. The implementation supports B-Trees auf order 3 or higher.
type bTrees struct {
	// byte slice for the ordering.
	p []byte

	// byte slice for access by _getLE64 for all valid positions
	_p []byte

	// hash table for the individual B-Trees.
	roots []*bNode

	// order of the B-Trees
	order int

	// shift value for the calculation of the hash value.
	shift uint
}

// m2 returns the ceiling of the order divided by 2.
func (t *bTrees) m2() int {
	return (t.order + 1) >> 1
}

// bNode represents a node in a B-Tree. We are not storing leaves in the
// tree. In a node that has leaves, the length of the children slice will be
// zero.
type bNode struct {
	keys     []uint32
	children []*bNode
}

// newBtrees creates a new B-Tree. The order must be larger than or equal 3. THe
// hashBits should be less or equal 24.
func newBTrees(order int, hashBits int, p []byte) *bTrees {
	if order < 3 {
		panic(fmt.Errorf("lz: order=%d; must be >= %d", order, 3))
	}
	if 0 < hashBits && hashBits < 24 {
		panic(fmt.Errorf("lz: hashBits=%d; must be >= %d", hashBits, 0))
	}
	if len(p)+7 > cap(p) {
		panic(fmt.Errorf("byte slice p has no 7-byte margin"))
	}
	return &bTrees{
		p:     p,
		_p:    p[:len(p)+7],
		roots: make([]*bNode, 1<<hashBits),
		order: order,
		shift: 64 - uint(hashBits),
	}
}

func (t *bTrees) hashValue(x uint64) uint32 {
	x &= btMask
	return uint32((x * prime) >> t.shift)
}

// add adds a position to the binary tree.
func (t *bTrees) add(pos uint32, x uint64) {
	pRoot := &t.roots[t.hashValue(x)]
	if *pRoot == nil {
		*pRoot = &bNode{keys: make([]uint32, 0, t.order-1)}
	}

	up, or := t.addAt(*pRoot, pos)
	if or == nil {
		return
	}
	root := &bNode{
		keys:     make([]uint32, 1, t.order-1),
		children: make([]*bNode, 2, t.order),
	}
	root.keys[0] = up
	root.children[0] = *pRoot
	root.children[1] = or
	*pRoot = root
}

// search searches for a position in the given node.
func (t *bTrees) search(o *bNode, pos uint32) int {
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

// addAt adds the position to the node o. If the node is split the node up with
// must be pushed higher and a new node is provided.
func (t *bTrees) addAt(o *bNode, pos uint32) (up uint32, or *bNode) {
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

func (t *bTrees) xValue(pos uint32) uint64 {
	return _getLE64(t._p[pos:]) & btMask
}

// addMax adds a new position under the assumption that the suffix starting at
// pos is larger than all suffixes added before.
func (t *bTrees) addMax(pos uint32) {
	x := t.xValue(pos)
	pRoot := &t.roots[t.hashValue(x)]
	if *pRoot == nil {
		*pRoot = &bNode{keys: make([]uint32, 0, t.order-1)}
	}
	up, or := t.addMaxAt(*pRoot, pos)
	if or == nil {
		return
	}
	root := &bNode{
		keys:     make([]uint32, 1, t.order-1),
		children: make([]*bNode, 2, t.order),
	}
	root.keys[0] = up
	root.children[0] = *pRoot
	root.children[1] = or
	*pRoot = root
}

// addMaxAt adds the a suffix starting at pos to the node under the assumption
// that the suffix is larger than all suffixes stored in the node.
func (t *bTrees) addMaxAt(o *bNode, pos uint32) (up uint32, or *bNode) {
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

func (t *bTrees) stealRight(o *bNode, i int) bool {
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

func (t *bTrees) stealLeft(o *bNode, i int) bool {
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

func (t *bTrees) dropKey(o *bNode, i int) {
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
func (t *bTrees) delMax(o *bNode) (r uint32, ok bool) {
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
func (t *bTrees) del(o *bNode, pos uint32) {
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

// delete removes the suffix starting at position pos from the B-Tree.
func (t *bTrees) delete(pos uint32) {
	x := t.xValue(pos)
	pRoot := &t.roots[t.hashValue(x)]
	if *pRoot == nil {
		return
	}
	root := *pRoot
	t.del(root, pos)
	switch len(root.children) {
	case 0:
		if len(root.keys) == 0 {
			*pRoot = nil
		}
	case 1:
		*pRoot = root.children[0]
	}
}

// walkNode calls function f in sequence of the sorted keys in the subtree
// starting at o.
func (t *bTrees) walkNode(o *bNode, f func([]uint32)) {
	if o == nil {
		return
	}
	if len(o.children) == 0 {
		f(o.keys)
		return
	}
	for i, c := range o.children {
		t.walkNode(c, f)
		if i < len(o.keys) {
			f(o.keys[i : i+1])
		}
	}
}

func (t *bTrees) addMaxRoot(root *bNode, pos uint32) *bNode {
	if root == nil {
		root = &bNode{keys: make([]uint32, 0, t.order-1)}
	}
	up, or := t.addMaxAt(root, pos)
	if or == nil {
		return root
	}
	r := &bNode{
		keys:     make([]uint32, 1, t.order-1),
		children: make([]*bNode, 2, t.order),
	}
	r.keys[0] = up
	r.children[0] = root
	r.children[1] = or
	return r
}

// adapt moves the content of the byte slices s bytes to the left and modifies
// the B-Tree accordingly. The current implementation recreates the B-Tree. Note
// that the shift in the slice must have been done, before calling adapt.
func (t *bTrees) adapt(s uint32) {
	for i, r := range t.roots {
		var root *bNode
		f := func(p []uint32) {
			for _, k := range p {
				if k < s {
					continue
				}
				root = t.addMaxRoot(root, k-s)
			}
		}
		t.walkNode(r, f)
		t.roots[i] = root
	}
}
