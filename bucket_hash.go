package lz

import (
	"fmt"
)

type bucketEntry struct {
	pos uint32
	val uint32
}

type bucketHash struct {
	buckets    []bucketEntry
	indexes    []byte
	mask       uint64
	shift      uint
	inputLen   int
	bucketSize int
}

func (bh *bucketHash) bucket(h uint32) []bucketEntry {
	k := int(h) * bh.bucketSize
	return bh.buckets[k : k+bh.bucketSize]
}

func (bh *bucketHash) add(h, pos, val uint32) {
	pi := &bh.indexes[h]
	i := int(*pi)
	k := int(h)*bh.bucketSize + i
	bh.buckets[k] = bucketEntry{pos, val}
	i++
	if i >= bh.bucketSize {
		i = 0
	}
	*pi = byte(i)
}

func (bh *bucketHash) init(inputLen, hashBits, bucketSize int) error {
	if !(2 <= inputLen && inputLen <= 8) {
		return fmt.Errorf("lz: inputLen must be in range [2,8]")
	}
	maxHashBits := 24
	if t := 8 * inputLen; t < maxHashBits {
		maxHashBits = t
	}
	if !(0 <= hashBits && hashBits <= maxHashBits) {
		return fmt.Errorf("lz: hashBits=%d; must be <= %d",
			hashBits, maxHashBits)
	}
	if !(1 <= bucketSize && bucketSize <= 128) {
		return fmt.Errorf("lz: bucketSize must be in the range [1,128]")
	}

	n := 1 << hashBits
	*bh = bucketHash{
		buckets:    make([]bucketEntry, n*bucketSize),
		indexes:    make([]byte, n),
		mask:       1<<(inputLen*8) - 1,
		shift:      64 - uint(hashBits),
		inputLen:   inputLen,
		bucketSize: bucketSize,
	}
	return nil
}

func (bh *bucketHash) reset() {
	for i := range bh.buckets {
		bh.buckets[i] = bucketEntry{}
	}
	for i := range bh.indexes {
		bh.indexes[i] = 0
	}
}

func (bh *bucketHash) adapt(delta uint32) {
	if delta == 0 {
		return
	}

	tmp := make([]bucketEntry, bh.bucketSize)
	for h, j := range bh.indexes {
		b := bh.bucket(uint32(h))
		i := 0
		for _, e := range b[j:] {
			if e.pos < delta {
				continue
			}
			e.pos -= delta
			tmp[i] = e
			i++
		}
		for _, e := range b[:j] {
			if e.pos < delta {
				continue
			}
			e.pos -= delta
			tmp[i] = e
			i++
		}
		copy(b, tmp[:i])
		if i >= bh.bucketSize {
			i = 0
		} else {
			p := b[i:]
			for k := range p {
				p[k] = bucketEntry{}
			}
		}
		bh.indexes[h] = byte(i)
	}
}
