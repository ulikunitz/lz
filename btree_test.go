package lz

import "testing"

func TestBtreeAdd(t *testing.T) {
	const s = `To be, or not to be`
	p := []byte(s)
	bt := newBtree(4, p)
	for i := 0; i < len(p); i++ {
		bt.add(uint32(i))
	}
	t.Logf("%v", bt)
}
