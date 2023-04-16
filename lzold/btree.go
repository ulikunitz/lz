package lzold

import (
	"bytes"
	"fmt"
	"io"
)

// bTree represents a B-tree as described by Donald Knuth. The slice p holds the
// data to compress and we store indexes to that array in the B-tree sorted by
// the suffixes starting at the key positions. Note that we are only supporting
// trees with order 3 or higher.
//
// The operations on the bTree are implement using a [bPath].
type bTree struct {
	pdata *[]byte
	root  *bNode
	order int

	// helper field used for debugging
	aux    int
	keybuf []uint32
}

// m2 returns the ceiling of the order divided by 2.
func (t *bTree) m2() int { return (t.order + 1) >> 1 }

// bNode represents a node in the B-tree. We are not storing leaves in the
// tree. In a node that has leaves the length of the children slice will be
// zero.
type bNode struct {
	keys     []uint32
	children []*bNode
}

// init initializes the tree structure
func (t *bTree) init(order int, pdata *[]byte) error {
	if order < 3 {
		return fmt.Errorf("lz: order=%d; must be >= %d", order, 3)
	}
	*t = bTree{
		pdata: pdata,
		order: order,
	}
	return nil
}

func (t *bTree) Reset(pdata *[]byte) {
	t.pdata = pdata
	t.root = nil
	t.aux = 0
}

func (t *bTree) setMatches(m int) error {
	if m < 0 {
		return fmt.Errorf("lz: matches=%d must not be negative", m)
	}
	t.keybuf = make([]uint32, m)
	return nil
}

// newBtree creates a new B-tree. The order must be larger than or equal 3.
func newBtree(order int, pdata *[]byte) *bTree {
	t := new(bTree)
	if err := t.init(order, pdata); err != nil {
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
func (t *bTree) Add(pos uint32, x uint64) {
	t._add(pos)
}

// search searches for a position in the given node.
func (t *bTree) search(o *bNode, pos uint32) int {
	p := *t.pdata
	q := p[pos:]
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

// stealRight checks whether it can steal a key from the right node.
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

// stealLeft checks if we can steal a key from the left node.
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

// dropKey drops a key and merges the right and left child node to ensure there
// are enough keys in the child. Note we must do stealing before to ensure that
// this works.
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

// delete deletes the position from the B-Tree. If the position is not part of
// the B-Tree the function does nothing.
func (t *bTree) delete(pos uint32) {
	var p bPath
	p.init(t)
	p.del(pos)
}

// adapt moves the content of the byte slices s bytes to the left and modifies
// the B-tree accordingly. The current implementation recreates the B-tree. Note
// that the shift in the slice must have been done, before calling adapt.
func (t *bTree) Adapt(s uint32) {
	var pt bPath
	pt.init(t)
	u := &bTree{order: t.order, pdata: t.pdata}
	var pu bPath
	pu.init(u)
	buf := make([]uint32, 16)
	for {
		n, err := pt.readKeys(buf)
		for _, k := range buf[:n] {
			if k >= s {
				pu.insertAfter(k - s)
			}
		}
		if err != nil {
			break
		}
	}
	t.root = u.root
}

func (t *bTree) AppendMatchesAndAdd(matches []uint32, pos uint32, x uint64) []uint32 {
	var p bPath
	p.init(t)
	p._search(pos)
	q := p.clone()
	q.back(len(t.keybuf) >> 1)
	n, _ := q.readKeys(t.keybuf)
	matches = append(matches, t.keybuf[:n]...)
	p._insert(pos)
	return matches
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

// subtree returns the root node of the subtree the path points to if it exists.
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
	t := p.t
	for {
		i := t.search(o, pos)
		p.append(o, i)
		if len(o.children) == 0 {
			return
		}
		o = o.children[i]
	}
}

// search looks for the pos value at the subtree described by the path and
// extends until it found pos or a position where it should be inserted. It
// returns whether the position has been found in the tree.
func (p *bPath) search(pos uint32) bool {
	o := p.subtree()
	if o == nil {
		return false
	}
	search := p.t.search
	for {
		i := search(o, pos)
		p.append(o, i)
		if i < len(o.keys) && o.keys[i] == pos {
			return true
		}
		if len(o.children) == 0 {
			return false
		}
		o = o.children[i]
	}
}

// clone creates a copy of the path.
func (p *bPath) clone() *bPath {
	s := make([]bEdge, len(p.s), cap(p.s))
	copy(s, p.s)
	return &bPath{s: s, t: p.t}
}

// append appends an edge to the path.
func (p *bPath) append(o *bNode, i int) {
	p.s = append(p.s, bEdge{o: o, i: i})
}

// max extends the path to the maximum element of the subtree the path points
// to.
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

// min extends the path to the minimum element of the subtree the path points
// to.
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

// next returns the path to the next key in sort order. The function returns
// an empty path at the end of the elements. The first element can be found with
// an empty path.
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

// prev returns the previous element in the sort order. For an empty path it
// returns the maximum element. For the minimum element the function returns the
// empty path.
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

// key returns the key the path points to.
func (p *bPath) key() (pos uint32, ok bool) {
	j := len(p.s) - 1
	if j < 0 {
		return 0, false
	}
	o, i := p.s[j].o, p.s[j].i
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
	o, i := p.s[j].o, p.s[j].i
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
		i -= k + 1
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
	for j--; j >= 0; j-- {
		o, i = p.s[j].o, p.s[j].i
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
			keys:     make([]uint32, kr, t.order-1),
			children: make([]*bNode, kr+1, t.order),
		}
		k -= kr
		copy(or.keys, o.keys[k:])
		o.keys = o.keys[:k]
		copy(or.children, o.children[k:])
		o.children = o.children[:k+1]
		switch {
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
		default: // i == k
			or.children[0] = ot
		}
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

// _insertPath adds pos32 before the position that p points to. Note that the path
// must always point to a node with leaves (len(o.children) == 0). In the
// difference to [bPath._insert] it updates the path to the position of the
// newly added item. Note that the position might be moved up in the tree, so
// you cannot call _insertPath usually after an insert.
func (p *bPath) _insertPath(pos uint32) {
	t := p.t
	j := len(p.s) - 1
	if j < 0 {
		t.root = &bNode{keys: make([]uint32, 1, t.order-1)}
		t.root.keys[0] = pos
		p.append(t.root, 0)
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
		i -= k + 1
		up = or.keys[0]
		copy(or.keys[:i], or.keys[1:])
		or.keys[i] = pos
		e.o, e.i = or, i
	case i < k:
		up = o.keys[k-1]
		copy(o.keys[i+1:], o.keys[i:])
		o.keys[i] = pos
	default: // i == k
		up = pos
		p.s = p.s[:k]
	}
	for j--; j >= 0; j-- {
		e = &p.s[j]
		o, i = e.o, e.i
		k = len(o.keys)
		if k+1 < t.order {
			o.keys = o.keys[:k+1]
			copy(o.keys[i+1:], o.keys[i:])
			o.keys[i] = up
			o.children = o.children[:len(o.children)+1]
			copy(o.children[i+2:], o.children[i+1:])
			o.children[i+1] = or
			if j+1 < len(p.s) && p.s[j+1].o == or {
				e.i++
			}
			return
		}
		pos = up
		kr = t.order >> 1
		ot := or
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
		case i > k:
			i -= k + 1
			up = or.keys[0]
			copy(or.keys[:i], or.keys[1:])
			or.keys[i] = pos
			copy(or.children[:i+1], or.children[1:])
			or.children[i+1] = ot
			e.o, e.i = or, i
			if j+1 < len(p.s) && p.s[j+1].o == ot {
				e.i++
			}
		case i < k:
			up = o.keys[k-1]
			copy(o.keys[i+1:], o.keys[i:])
			o.keys[i] = pos
			copy(o.children[i+2:], o.children[i+1:])
			o.children[i+1] = ot
			if j+1 < len(p.s) && p.s[j+1].o == ot {
				e.i++
			}
		default: // i == k
			or.children[0] = ot
			if j+1 < len(p.s) {
				if p.s[j+1].o == ot {
					e.o, e.i = or, 0
				}
			} else {
				p.s = p.s[:j]
			}
		}
	}
	root := &bNode{
		keys:     make([]uint32, 1, t.order-1),
		children: make([]*bNode, 2, t.order),
	}
	root.keys[0] = up
	root.children[0] = t.root
	root.children[1] = or
	t.root = root
	j = len(p.s) + 1
	if j > cap(p.s) {
		s := make([]bEdge, j)
		copy(s[1:], p.s)
		p.s = s
	} else {
		p.s = p.s[:j]
		copy(p.s[1:], p.s)
	}
	e = &p.s[0]
	e.o, e.i = root, 0
	if 1 < len(p.s) && p.s[1].o == or {
		e.i = 1
	}
}

// nextInsert returns the next element to use the insert functions for
// insertAfter.
func (p *bPath) nextInsert() {
	j := len(p.s) - 1
	if j < 0 {
		p.min()
		return
	}
	e := &p.s[j]
	o, i := e.o, e.i
	if len(o.children) == 0 {
		if i < len(o.keys) {
			e.i++
		}
		return
	}
	if i < len(o.keys) {
		e.i++
	}
	p.min()
}

// insertAfter inserts pos after where the path points to.
func (p *bPath) insertAfter(pos uint32) {
	p.nextInsert()
	p._insertPath(pos)
}

// del removes the pos from the subtree the path points to. The path is
// undefined after deletion.
func (p *bPath) del(pos uint32) {
	if found := p.search(pos); !found {
		return
	}
	d := len(p.s) - 1
	o, i := p.s[d].o, p.s[d].i
	var j int
	if len(o.children) > 0 {
		p.max()
		o.keys[i], _ = p.key()
		j = len(p.s) - 1
		o, i = p.s[j].o, p.s[j].i
	} else {
		j = d
	}
	copy(o.keys[i:], o.keys[i+1:])
	o.keys = o.keys[:len(o.keys)-1]
	t := p.t
	for {
		if len(o.keys)+1 >= t.m2() {
			return
		}
		j--
		if j < 0 {
			break
		}
		o, i = p.s[j].o, p.s[j].i
		if j <= d && t.stealRight(o, i) {
			return
		}
		if t.stealLeft(o, i) {
			return
		}
		if i >= len(o.keys) {
			i--
		}
		t.dropKey(o, i)
	}
	switch len(t.root.children) {
	case 0:
		if len(t.root.keys) == 0 {
			t.root = nil
		}
	case 1:
		t.root = t.root.children[0]
	}
}

// back tries to jump back n keys. If the end is reached [io.EOF] will be
// returned and the number of keys that have been jumped over.
func (p *bPath) back(n int) (k int, err error) {
	if n == 0 {
		return 0, nil
	}
	if len(p.s) == 0 {
		p.prev()
		if len(p.s) == 0 {
			return 0, io.EOF
		}
		k++
	}
	for k < n {
		e := &p.s[len(p.s)-1]
		o, i := e.o, e.i
		if len(o.children) > 0 || i == 0 {
			p.prev()
			if len(p.s) == 0 {
				return k, io.EOF
			}
			k++
			continue
		}
		d := i - (n - k)
		if d >= 0 {
			e.i = d
			break
		}
		e.i = 0
		k += i
	}
	return n, nil
}

// readKeys reads keys from the tree in sorted order starting with the position
// the path points to. It returns io.EOF once. A repeat call will start reading
// the keys again.
func (p *bPath) readKeys(s []uint32) (n int, err error) {
	if len(p.s) == 0 {
		p.next()
	}
	for {
		if len(p.s) == 0 {
			return n, io.EOF
		}
		if n == len(s) {
			return n, nil
		}
		e := &p.s[len(p.s)-1]
		o, i := e.o, e.i
		switch {
		case i == len(o.keys):
			p.next()
		case len(o.children) == 0:
			k := copy(s[n:], o.keys[i:])
			n += k
			e.i += k
		default:
			s[n] = o.keys[i]
			n++
			p.next()
		}
	}
}

type BTreeConfig struct {
	Order   int
	Matches int
}

func (cfg *BTreeConfig) Verify() error {
	if cfg.Order < 3 {
		return fmt.Errorf("lz: Order must be >= 3")
	}
	if cfg.Matches < 0 {
		return fmt.Errorf("lz: Matches must be >= 0")
	}
	return nil
}

func (cfg *BTreeConfig) ApplyDefaults() {
	if cfg.Order == 0 {
		cfg.Order = 128
	}
	if cfg.Matches == 0 {
		cfg.Matches = 2
	}
}

func (cfg *BTreeConfig) NewMatchFinder() (mf MatchFinder, err error) {
	cfg.ApplyDefaults()
	if err = cfg.Verify(); err != nil {
		return nil, err
	}
	bt := newBtree(cfg.Order, nil)
	if err = bt.setMatches(cfg.Matches); err != nil {
		return nil, err
	}

	return bt, nil
}
