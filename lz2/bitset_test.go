package lz2

import (
	"fmt"
	"testing"
)

func TestBitsetSimple(t *testing.T) {
	var b bitset
	if b.member(10) {
		t.Fatalf(
			"b.member(10) returned true; want false")
	}
	b.insert(10)
	if !b.member(10) {
		t.Fatalf(
			"b.member(10) returned false; want true")
	}
	_, ok := b.memberBefore(10)
	if ok {
		t.Fatalf("b.memberBefore(%d) returns %t; want false",
			10, ok)
	}
	var k int
	k, ok = b.memberBefore(11)
	if !ok {
		t.Fatalf("b.memberBefore(%d) returns %t; want true",
			11, ok)
	}
	if k != 10 {
		t.Fatalf("b.memberBefore(%d) returns %d; want %d",
			11, k, 10)
	}
	k, ok = b.memberBefore(129)
	if !ok {
		t.Fatalf("b.memberBefore(%d) returns %t; want true",
			129, ok)
	}
	if k != 10 {
		t.Fatalf("b.memberBefore(%d) returns %d; want %d",
			129, k, 10)
	}
	if n := b.pop(); n != 1 {
		t.Fatalf("b.pop() returns %d; want %d", n, 1)
	}

	b.clear()
	if b.pop() != 0 {
		t.Fatalf("b.clear() didn't clear")
	}

	b.insert(127)

	if !b.member(127) {
		t.Fatalf("b.member(%d) is false; want true", 127)
	}

	_, ok = b.memberAfter(127)
	if ok {
		t.Fatalf("b.memberAfter(%d) is true; want false", 127)
	}

	k, ok = b.memberAfter(126)
	if !ok {
		t.Fatalf("b.memberAfter(%d) is false; want true", 126)
	}
	if k != 127 {
		t.Fatalf("b.memberAfter(%d) returns %d; want %d", 126, k, 127)
	}

	k, ok = b.memberAfter(0)
	if !ok {
		t.Fatalf("b.memberAfter(%d) is false; want true", 0)
	}
	if k != 127 {
		t.Fatalf("b.memberAfter(%d) returns %d; want %d", 0, k, 127)
	}
}

func equalIntSlices(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i, x := range a {
		if x != b[i] {
			return false
		}
	}
	return true
}

func TestBitsetSimple2(t *testing.T) {
	var a bitset
	if a.member(64) {
		t.Fatalf("a.member(%d) returns true; want false", 64)
	}
	_, ok := a.firstMember()
	if ok {
		t.Fatalf("a.firstMember returns true; want false")
	}
	a.insert(64)
	if !a.member(64) {
		t.Fatalf("a.member(%d) returns false; want true", 64)
	}
	var i int
	i, ok = a.firstMember()
	if !ok {
		t.Fatalf("a.firstMember() returns false; want true")
	}
	if i != 64 {
		t.Fatalf("a.firstMember() returns %d; want %d", i, 64)
	}
	a.clear()
	_, ok = a.firstMember()
	if ok {
		t.Fatalf("a.firstMember returns true; want false")
	}
	w := []int{1, 2, 3, 63, 64, 127, 128}
	a.insert(w...)
	s := a.slice()
	if !equalIntSlices(s, w) {
		t.Fatalf("a.slice() returned %d; want %d", s, w)
	}
	u1 := []int{1, 2, 3, 63, 64, 65, 127, 128, 129}
	u2 := []int{65, 66, 230, 16500}
	v := []int{65}
	var a1, a2, b bitset
	a1.insert(u1...)
	a2.insert(u2...)
	v2 := a2.slice()
	if !equalIntSlices(v2, u2) {
		t.Fatalf("v2=%d; want u2=%d", v2, u2)
	}
	a2str := a2.String()
	const wa2str = "{65, 66, 230, 16500}"
	if a2str != wa2str {
		t.Fatalf("a2.String() returned %q; want %q", a2str, wa2str)
	}
	b.intersect(&a1, &a2)
	u := b.slice()
	if !equalIntSlices(u, v) {
		t.Fatalf("b.intersect(&a1,&a2) is %d; want %d", u, v)
	}
	k, ok := a2.memberAfter(230)
	if !ok {
		t.Fatalf("a2.memberAfter(%d) returned false; want true", 230)
	}
	if k != 16500 {
		t.Fatalf("a2.memberAfter(%d) returned %d; want %d",
			230, k, 16500)
	}
	_, ok = a2.memberAfter(16500)
	if ok {
		t.Fatalf("a2.memberAfter(%d) returned true; want false", 16500)
	}
}

func TestBitsetIntersect(t *testing.T) {
	tests := []struct {
		x, y, z  []int
		lenSlice int
	}{
		{[]int{0, 64, 128, 256}, []int{1, 64, 129, 257}, []int{64}, 1},
		{nil, nil, nil, 0},
		{[]int{0, 1}, []int{3000}, nil, 0},
	}
	for i, tc := range tests {
		t.Run(fmt.Sprintf("#%d", i+1), func(t *testing.T) {
			var x, y, z bitset
			x.insert(tc.x...)
			y.insert(tc.y...)
			z.intersect(&x, &y)
			got := z.slice()
			if !equalIntSlices(got, tc.z) {
				var wz bitset
				wz.insert(tc.z...)
				t.Errorf("intersect(%s,%s) got %s; want %s",
					&x, &y, &z, &wz)
			}
			if len(z.a) != tc.lenSlice {
				t.Errorf("intersect(%s,%s) len(z.a)=%d; want %d",
					&x, &y, len(z.a), tc.lenSlice)

			}
		})
	}
}

func TestBitsetDelete(t *testing.T) {
	var b bitset
	b.insert(1, 2, 65)
	b.delete(3)
	if len(b.a) != 2 {
		t.Fatalf("b.delete(%d): len(b.a)=%d; want %d",
			3, len(b.a), 2)
	}
	b.delete(65)
	if len(b.a) != 1 {
		t.Fatalf("b.delete(%d): len(b.a)=%d; want %d",
			65, len(b.a), 1)
	}
	s := b.slice()
	w := []int{1, 2}
	if !equalIntSlices(w, s) {
		t.Fatalf("b.delete(%d) b=%d; want %d", 65, s, w)
	}
}

func TestBitsetString(t *testing.T) {
	var b bitset
	b.insert(1, 3, 5, 129)
	s := b.String()
	const want = "{1, 3, 5, 129}"
	if s != want {
		t.Fatalf("b.String() returned %q; want %q", s, want)
	}
}
