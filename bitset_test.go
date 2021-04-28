package lz

import "testing"

func TestBitsetSimple(t *testing.T) {
	var b bitset
	b.init(130)
	if b.isMember(10) {
		t.Fatalf(
			"b.IsMember(10) returned true; want false")
	}
	b.insert(10)
	if !b.isMember(10) {
		t.Fatalf(
			"b.IsMember(10) returned false; want true")
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

	if !b.isMember(127) {
		t.Fatalf("b.isMember(%d) is false; want true", 127)
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
