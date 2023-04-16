package lzold

/*

type repeat4 [4]uint32

func (r *repeat4) find(p []byte, i int) (o int, k int) {
	for _, off := range *r {
		j := i - int(off)
		if !(0 <= j && j < i) {
			continue
		}
		l := matchLen(p[j:], p[i:])
		if l > k {
			o, k = int(off), l
		}
	}
	return o, k
}

func (r *repeat4) pick(o int) {
	switch off := uint32(o); off {
	default:
		r[3] = r[2]
		fallthrough
	case r[2]:
		r[2] = r[1]
		fallthrough
	case r[1]:
		r[1] = r[0]
		r[0] = off
	case r[0]:
	}
}

*/
