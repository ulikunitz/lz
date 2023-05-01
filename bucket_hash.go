package lz

import (
	"errors"
	"fmt"
	"reflect"
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

type bucketConfig struct {
	InputLen   int
	HashBits   int
	BucketSize int
}

var errNoBucketConfig = errors.New("lz: no bucket hash configuration")

func bucketCfg(cfg ParserConfig) (b bucketConfig, err error) {
	v := reflect.Indirect(reflect.ValueOf(cfg))
	f := hasVal(v, "InputLen")
	f = f && hasVal(v, "HashBits")
	f = f && hasVal(v, "BucketSize")
	if !f {
		return bucketConfig{}, errNoBucketConfig
	}
	b.InputLen = iVal(v, "InputLen")
	b.HashBits = iVal(v, "HashBits")
	b.BucketSize = iVal(v, "BucketSize")
	return b, nil
}

func setBucketCfg(cfg ParserConfig, b bucketConfig) error {
	v := reflect.Indirect(reflect.ValueOf(cfg))
	f := hasVal(v, "InputLen")
	f = f && hasVal(v, "HashBits")
	f = f && hasVal(v, "BucketSize")
	if !f {
		return errNoBucketConfig
	}
	setIVal(v, "InputLen", b.InputLen)
	setIVal(v, "HashBits", b.HashBits)
	setIVal(v, "BucketSize", b.HashBits)
	return nil
}

func (cfg *bucketConfig) SetDefaults() {
	if cfg.InputLen == 0 {
		cfg.InputLen = 3
	}
	if cfg.HashBits == 0 {
		cfg.HashBits = 12
	}
	if cfg.BucketSize == 0 {
		cfg.BucketSize = 10
	}
}

func (cfg *bucketConfig) Verify() error {
	if !(2 <= cfg.InputLen && cfg.InputLen <= 8) {
		return fmt.Errorf(
			"lz: InputLen=%d; must be in range [2,8]", cfg.InputLen)
	}
	// We want to reduce the hash table size, which may lead to
	// out-of-memory conditions.
	maxHashBits := 23
	if t := 8 * cfg.InputLen; t < maxHashBits {
		maxHashBits = t
	}
	if !(0 <= cfg.HashBits && cfg.HashBits <= maxHashBits) {
		return fmt.Errorf("lz: HashBits=%d; must be <= %d",
			cfg.HashBits, maxHashBits)
	}
	if !(1 <= cfg.BucketSize && cfg.BucketSize <= 128) {
		return fmt.Errorf("lz: BucketSize=%d; must be in range [1,128]",
			cfg.BucketSize)
	}
	return nil
}

func (bh *bucketHash) init(cfg *bucketConfig) error {
	cfg.SetDefaults()
	var err error
	if err = cfg.Verify(); err != nil {
		return err
	}

	n := 1 << cfg.HashBits
	*bh = bucketHash{
		buckets:    make([]bucketEntry, n*cfg.BucketSize),
		indexes:    make([]byte, n),
		mask:       1<<(cfg.InputLen*8) - 1,
		shift:      64 - uint(cfg.HashBits),
		inputLen:   cfg.InputLen,
		bucketSize: cfg.BucketSize,
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

func (bh *bucketHash) shiftOffsets(delta uint32) {
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

type bucketDictionary struct {
	ParserBuffer
	bucketHash
}

func (f *bucketDictionary) init(cfg bucketConfig, bcfg BufConfig) error {
	var err error
	if err = f.ParserBuffer.Init(bcfg); err != nil {
		return err
	}
	cfg.SetDefaults()
	if err = cfg.Verify(); err != nil {
		return err
	}
	err = f.bucketHash.init(&cfg)
	return err
}

func (f *bucketDictionary) Reset(data []byte) error {
	var err error
	if err = f.ParserBuffer.Reset(data); err != nil {
		return err
	}
	f.bucketHash.reset()
	return nil
}

func (f *bucketDictionary) Shrink() int {
	delta := f.ParserBuffer.Shrink()
	if delta > 0 {
		f.bucketHash.shiftOffsets(uint32(delta))
	}
	return delta
}

func (f *bucketDictionary) processSegment(a, b int) {
	if a < 0 {
		a = 0
	}
	c := len(f.Data) - f.inputLen + 1
	if c < b {
		b = c
	}
	if b <= 0 {
		return
	}

	_p := f.Data[:b+7]
	for i := a; i < b; i++ {
		x := _getLE64(_p[i:]) & f.mask
		f.add(hashValue(x, f.shift), uint32(i), uint32(x))
	}
}
