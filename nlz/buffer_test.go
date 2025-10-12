package nlz

import (
	"strings"
	"testing"
)

func TestBufferReadFrom(t *testing.T) {
	const str = "The quick brown fox jumps over the lazy dog."
	var buf Buffer
	buf.Init(8)

	r := strings.NewReader(str)
	for {
		n, err := buf.ReadFrom(r)
		if n == 0 {
			if err == nil {
				break
			}
			if err != ErrFullBuffer {
				t.Fatalf("ReadFrom error %s", err)
			}
		}
		buf.W = len(buf.Data)
		k := buf.Prune(4)
		if k == 0 {
			t.Fatalf("Prune returned k=0")
		}
		if k > 4 {
			t.Fatalf("Prune returned k=%d; want <= 4", k)
		}
	}
	if cap(buf.Data) > buf.Size+7 {
		t.Fatalf("cap(buf.Data) is %d; want <= %d",
			cap(buf.Data), buf.Size+7)
	}
}

func TestBufferWrite(t *testing.T) {
	const str = "The quick brown fox jumps over the lazy dog."
	var buf Buffer
	buf.Init(8)

	p := []byte(str)
	for len(p) > 0 {
		n, err := buf.Write(p)
		if n < len(p) && err == nil {
			t.Fatalf(
				"Write returned n=%d, err=nil; want non-nil err",
				n)
		}
		p = p[n:]

		buf.W = len(buf.Data)
		k := buf.Prune(4)
		if k == 0 {
			t.Fatalf("Prune returned k=0")
		}
		if k > 4 {
			t.Fatalf("Prune returned k=%d; want <= 4", k)
		}
	}
	if cap(buf.Data) > buf.Size+7 {
		t.Fatalf("cap(buf.Data) is %d; want <= %d",
			cap(buf.Data), buf.Size+7)
	}
}
