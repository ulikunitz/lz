package lz

// A matchFinder is used to find potential matches.
type MatchFinder interface {
	// Update informs the match finder to data changes in the data slice. If
	// delta is less than zero than complete new data is provided. If the
	// delta is positive data has been moved delta bytes down in the slice.
	// If delta is zero data has been added.
	Update(data []byte, delta int)

	// Process segment puts all hashes into the has table unless there is
	// not enough enough data at the end.
	ProcessSegment(a, b int)

	// AppendMatchOffsets looks for potential matches and adds the position
	// i to the finder.
	AppendMatchOffsets(m []uint32, i int) []uint32
}
