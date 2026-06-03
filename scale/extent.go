package scale

import "math"

// Extent returns the minimum and maximum of the given values, skipping NaN
// values. For empty input (no non-NaN values), it returns (0, 0).
func Extent(values []float64) (lo, hi float64) {
	if len(values) == 0 {
		return 0, 0
	}
	lo = math.Inf(1)
	hi = math.Inf(-1)
	for _, v := range values {
		if math.IsNaN(v) {
			continue
		}
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	if math.IsInf(lo, 1) {
		// All values were NaN
		return 0, 0
	}
	return lo, hi
}

// ExtentBy returns the minimum and maximum of the accessor function applied
// to each item, skipping NaN results. For empty input (no items or all NaN
// results), it returns (0, 0).
func ExtentBy[T any](items []T, accessor func(T) float64) (lo, hi float64) {
	if len(items) == 0 {
		return 0, 0
	}
	lo = math.Inf(1)
	hi = math.Inf(-1)
	for _, item := range items {
		v := accessor(item)
		if math.IsNaN(v) {
			continue
		}
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	if math.IsInf(lo, 1) {
		return 0, 0
	}
	return lo, hi
}

// NiceExtent computes the extent of the given values and rounds it outward
// to human-friendly tick boundaries. This is a convenience combining Extent
// and Nice in one call.
func NiceExtent(values []float64, count int) (lo, hi float64) {
	lo, hi = Extent(values)
	if len(values) == 0 {
		return lo, hi
	}
	if lo == hi && count > 0 && !math.IsNaN(lo) && !math.IsInf(lo, 0) {
		if lo == 0 {
			hi = 1
		} else {
			lo, hi = lo-math.Abs(lo)*0.1, hi+math.Abs(hi)*0.1
		}
	}
	return Nice(lo, hi, count)
}
