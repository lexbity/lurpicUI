package scale

// lerp linearly interpolates between a and b by t. When t=0 the result is a;
// when t=1 the result is b. t is not clamped to [0,1]; values outside that
// interval produce extrapolation. Total: never panics.
func lerp(a, b, t float64) float64 {
	return a + (b-a)*t
}

// normalize maps x from the interval [lo, hi] to [0,1].
// For degenerate spans (lo == hi) it returns 0 to avoid division by zero.
// Total: never panics.
func normalize(x, lo, hi float64) float64 {
	if lo == hi {
		return 0
	}
	return (x - lo) / (hi - lo)
}

// uninterpolate returns a function that maps values from [lo, hi] to [0,1].
// For degenerate spans (lo == hi) the returned function always returns 0.
func uninterpolate(lo, hi float64) func(float64) float64 {
	if lo == hi {
		return func(float64) float64 { return 0 }
	}
	return func(x float64) float64 {
		return (x - lo) / (hi - lo)
	}
}

// clamp01 clamps v to the unit interval [0, 1].
// Total: never panics.
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
