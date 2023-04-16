package lzold

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"reflect"
	"strings"
	"testing"
)

func fprintNode(w io.Writer, o *bNode) {
	if o == nil {
		fmt.Fprintf(w, "(nil)")
		return
	}
	if len(o.children) == 0 {
		fmt.Fprint(w, "[")
		for i, k := range o.keys {
			if i > 0 {
				fmt.Fprint(w, " ")
			}
			fmt.Fprintf(w, "%d", k)
		}
		fmt.Fprint(w, "]")
		return
	}

	fmt.Fprint(w, "[")
	for i, c := range o.children {
		fprintNode(w, c)
		if i < len(o.keys) {
			fmt.Fprintf(w, " %d ", o.keys[i])
		}
	}
	fmt.Fprint(w, "]")
}

func sprintNode(o *bNode) string {
	var sb strings.Builder
	fprintNode(&sb, o)
	return sb.String()
}

func appendNode(p []uint32, o *bNode) []uint32 {
	if o == nil {
		return p
	}
	if len(o.children) == 0 {
		return append(p, o.keys...)
	}
	for i, k := range o.keys {
		p = appendNode(p, o.children[i])
		p = append(p, k)
	}
	p = appendNode(p, o.children[len(o.children)-1])
	return p
}

func TestBTreeAdd(t *testing.T) {
	const s = "To be, or not to be"
	tests := []int{3, 4, 5, 6, 10, 15, 20}
	for _, tc := range tests {
		tc := tc
		t.Run(fmt.Sprintf("%d", tc), func(t *testing.T) {
			p := []byte(s)
			bt := newBtree(tc, &p)
			for i := 0; i < len(p); i++ {
				t.Logf("btree#%d %s",
					tc, sprintNode(bt.root))
				if err := verifyBTree(bt); err != nil {
					t.Fatalf("verifyBtree error %s", err)
				}
				bt._add(uint32(i))
			}
			q := appendNode(nil, bt.root)
			for i := 1; i < len(q); i++ {
				if !(bytes.Compare(p[q[i-1]:], p[q[i]:]) < 0) {
					t.Fatalf("p[%d@%d:]=%q >= p[%d@%d:]=%q",
						q[i-1], i-1, p[q[i-1]:],
						q[i], i, p[q[i]:])
				}
			}
			t.Logf("%d", q)
		})
	}
}

func (t *bTree) verifyNode(o *bNode, depth int) error {
	if o == nil {
		return nil
	}

	// compute the number of children.
	k := len(o.children)
	if k == 0 {
		k = len(o.keys) + 1
	}

	// We are checking Knuth's properties.

	// i) Every node has at most $m$ children.
	//    Since we are not storing leaves we have to take that into account.
	if k > t.order {
		return fmt.Errorf("i) k=%d > m=%d; %s", k, t.order,
			sprintNode(o))
	}

	// ii) Every node, except for the root and the leaves, has at most m/2
	//     children.
	m2 := (t.order + 1) >> 1
	if o != t.root && k < m2 {
		return fmt.Errorf("ii) k=%d < ceil(m/2)=%d; %s", k, m2,
			sprintNode(o))
	}

	// iii) The root has at least 2 children (unless it is a leaf).
	if o == t.root && k < 2 {
		return fmt.Errorf("iii) k=%d < 2 for root; %s", len(o.children),
			sprintNode(o))
	}

	// iv) All leaves appear on the same level and carry no information.
	//     Can't test it without additional information.
	if len(o.children) == 0 {
		if t.aux < 0 {
			t.aux = depth
		} else if t.aux != depth {
			return fmt.Errorf("iv) depth=%d; expected %d; %s",
				depth, t.aux, sprintNode(o))
		}
	}

	// v) A nonleaf node with k children contains k-1 keys.
	if len(o.keys) != k-1 {
		return fmt.Errorf("v) len(o.keys)=%d != (k=%d)-1; %s",
			len(o.keys), k, sprintNode(o))
	}

	// Check all children.
	for _, child := range o.children {
		if err := t.verifyNode(child, depth+1); err != nil {
			return err
		}
	}
	return nil
}

func verifyBTree(t *bTree) error {
	if t == nil {
		return fmt.Errorf("lz.bTree: is nil; must be non-nil")
	}
	if t.pdata == nil {
		return fmt.Errorf("lz.bTree: pdata is nil; must be non-nil")
	}
	p := *t.pdata
	if len(p) > math.MaxUint32 {
		return fmt.Errorf("lz.bTree: len(p) is %d; must be less than MaxUint32=%d",
			len(p), math.MaxUint32)
	}
	if !(t.order >= 2) {
		return fmt.Errorf("lz.bTree: t.order is %d; must be >= %d",
			t.order, 2)
	}
	if t.root != nil {
		t.aux = -1
		if err := t.verifyNode(t.root, 0); err != nil {
			return err
		}
	}
	s := appendNode(nil, t.root)
	for i := 0; i < len(s); i++ {
		if i > 0 && bytes.Compare(p[s[i-1]:], p[s[i]:]) >= 0 {
			return fmt.Errorf(
				"lz.bTree: wrong keys order s[%d]=%d is large or equal s[%d]=%d",
				i-1, s[i-1], i, s[i])
		}
	}

	return nil
}

func TestBTreeDel(t *testing.T) {
	const s = "To be, or not to be"
	tests := []int{3, 4, 5, 6, 10, 15, 20}
	for _, tc := range tests {
		tc := tc
		t.Run(fmt.Sprintf("%d", tc), func(t *testing.T) {
			p := []byte(s)
			bt := newBtree(tc, &p)
			if err := verifyBTree(bt); err != nil {
				t.Fatalf("verifyBtree error %s", err)
			}
			for i := 0; i < len(p); i++ {
				bt._add(uint32(i))
				if err := verifyBTree(bt); err != nil {
					t.Fatalf("add(%d) - verifyBtree error %s",
						i, err)
				}
			}
			t.Logf("full tree %s", sprintNode(bt.root))
			for i := len(p) - 1; i >= 0; i-- {
				bt.delete(uint32(i))
				t.Logf("after bt.delete(%d) %s", i,
					sprintNode(bt.root))
				if err := verifyBTree(bt); err != nil {
					t.Fatalf("delete(%d) - verifyBtree error %s",
						i, err)
				}
			}
			q := appendNode(nil, bt.root)
			if len(q) != 0 {
				t.Fatalf("got %d after deleting all positions",
					q)
			}
		})
	}

}

func TestBTreeAdapt(t *testing.T) {
	const s = "To be, or not to be"
	tests := []int{3, 4, 5, 6, 10, 15, 20}
	for _, tc := range tests {
		tc := tc
		t.Run(fmt.Sprintf("%d", tc), func(t *testing.T) {
			p := []byte(s)
			bt := newBtree(tc, &p)
			if err := verifyBTree(bt); err != nil {
				t.Fatalf("verifyBtree error %s", err)
			}
			for i := 0; i < len(p); i++ {
				bt._add(uint32(i))
				if err := verifyBTree(bt); err != nil {
					t.Fatalf("add(%d) - verifyBtree error %s",
						i, err)
				}
			}
			origKeys := appendNode(nil, bt.root)
			t.Logf("origKeys: *%d%d", len(origKeys), origKeys)
			const delta = 7
			k := copy(p, p[delta:])
			p = p[:k]
			bt.Adapt(delta)
			if err := verifyBTree(bt); err != nil {
				t.Fatalf("bt.adapt(%d) - verifyBTree error %s",
					delta, err)
			}
			keys := appendNode(nil, bt.root)
			t.Logf("bt.adapt(%d) -> *%d%d", delta, len(keys), keys)
			if len(keys) != len(p) {
				t.Fatalf("bt.adapt(%d): len(keys)=%d; want %d",
					delta, len(keys), len(p))
			}
			for k, i := range keys[:len(keys)-1] {
				j := keys[k+1]
				if bytes.Compare(p[i:], p[j:]) >= 0 {
					t.Fatalf("p[%d:]=%q >= p[%d:]=%q",
						i, j, p[i:], p[j:])
				}
			}
		})
	}
}

func TestBTreePathMethods(t *testing.T) {
	const str = "To be, or not to be"
	tests := []int{3, 4, 5, 6, 10, 15, 20}
	for _, tc := range tests {
		tc := tc
		t.Run(fmt.Sprintf("%d", tc), func(t *testing.T) {
			p := []byte(str)
			bt := newBtree(tc, &p)
			for i := range p {
				bt._add(uint32(i))
			}
			var bp bPath
			bp.init(bt)
			s := make([]uint32, len(p))
			n, _ := bp.readKeys(s)
			if n != len(p) {
				t.Fatalf("bp.readKeys(s) returned %d; want %d",
					n, len(p))
			}
			t.Logf("suffix array %d", s)
			for k, i := range s[:len(s)-1] {
				j := s[k+1]
				if bytes.Compare(p[i:], p[j:]) > 0 {
					t.Fatalf("p[%d:]=%q > p[%d:]=%q",
						i, p[i:], j, p[j:])
				}
			}
			for _, k := range s {
				t.Logf("%q", p[k:])
			}
		})
	}
}

func TestBTreeBack(t *testing.T) {
	const str = "To be, or not to be"
	tests := []int{3, 4, 5, 6, 10, 15, 20}
	for _, tc := range tests {
		tc := tc
		t.Run(fmt.Sprintf("%d", tc), func(t *testing.T) {
			p := []byte(str)
			bt := newBtree(tc, &p)
			for i := range p {
				bt._add(uint32(i))
			}
			var bp bPath
			bp.init(bt)
			s := make([]uint32, len(p))
			k, _ := bp.readKeys(s)
			if k != len(p) {
				t.Fatalf("bp.readKeys returned %d; want %d", k, len(p))
			}
			t.Logf("keys: %d", s)
			bp.reset()

			n := 0
			for {
				k, err := bp.back(2)
				n += k
				if !(0 <= k && k <= 2) {
					t.Fatalf("bp.back(%d) returned k=%d",
						2, k)
				}
				if err == io.EOF {
					break
				}
			}
			if n != len(p) {
				t.Fatalf(
					"bp.back() returned %d in total; want %d",
					n, len(p))
			}
		})
	}
}

func TestBTreeAppendMatchesAndAdd(t *testing.T) {
	//           0123456789012345678
	const str = "To be, or not to be"
	tests := []int{3, 4, 5, 6, 10, 15, 20}
	for _, tc := range tests {
		tc := tc
		t.Run(fmt.Sprintf("%d", tc), func(t *testing.T) {
			p := []byte(str)
			bt := newBtree(tc, &p)
			bt.setMatches(2)
			for i := range p[:14] {
				bt._add(uint32(i))
			}
			const pos = 15
			w := []uint32{10, 1}
			matches := bt.AppendMatchesAndAdd(nil, pos,
				getLE64(p[pos:]))
			if !reflect.DeepEqual(matches, w) {
				t.Fatalf("bt.appendMatchesAndAdd returned %d; want %d",
					matches, w)
			}
			t.Logf("matches: %d", matches)
		})
	}
}
