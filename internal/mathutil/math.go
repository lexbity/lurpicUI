package mathutil

// Min returns the smaller of a and b.
func Min[T ~float32 | ~float64 | ~int | ~int64](a, b T) T {
	if a < b {
		return a
	}
	return b
}

// Max returns the larger of a and b.
func Max[T ~float32 | ~float64 | ~int | ~int64](a, b T) T {
	if a > b {
		return a
	}
	return b
}
