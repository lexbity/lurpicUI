package scale

// clamp constrains v to the interval [lo, hi]. When lo > hi the result is
// implementation-defined but the function never panics.
// Total: never panics.
func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// clampOutOfRange applies the OutOfRange policy to a mapped value.
// When policy is OutOfRangeClamp, value is clamped to [lo, hi].
// When policy is OutOfRangeExtrapolate, value is returned unchanged.
// Total: never panics.
func clampOutOfRange(value, lo, hi float64, policy OutOfRange) float64 {
	if policy == OutOfRangeClamp {
		return clamp(value, lo, hi)
	}
	return value
}
