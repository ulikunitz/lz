package lz

// A matchFinder is used to find potential matches.
type MatchFinder interface {
	// Resets the data slice to search for matches and applies the delta. If
	// delta is zero no data has been retained from the last data slice
	// provided.
	Reset(data []byte, delta int)

	// Process segment puts all hashes into the has table unless there is
	// not enough enough data at the end.
	ProcessSegment(a, b int)

	// AppendMatchOffsets looks for potential matches and adds the position
	// i to the finder.
	AppendMatchOffsets(m []uint32, i int) []uint32
}
