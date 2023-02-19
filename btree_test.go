package lz

import (
	"bytes"
	"fmt"
	"testing"
)

func appendNode(p []uint32, o *bNode) []uint32 {
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
	const s = `To be, or not to be`
	tests := []int{2, 3, 4, 5, 6, 10, 15, 20}
	for _, tc := range tests {
		tc := tc
		t.Run(fmt.Sprintf("%d", tc), func(t *testing.T) {
			p := []byte(s)
			bt := newBtree(tc, p)
			for i := 0; i < len(p); i++ {
				bt.add(uint32(i))
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
