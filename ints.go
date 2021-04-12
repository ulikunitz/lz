package lz

// iverson converts a bool into an integer. The function will be inlined without
// without jumps.
func iverson64(f bool) int64 {
	if f {
		return 1
	}
	return 0
}

// doz computes x-y if x > y or 0 if x <= y.
func doz64(x, y int64) int64 {
	return (x - y) & (-iverson64(x >= y))
}
