package scale

import "math"

// PanDomain pans the domain [lo, hi] by delta in domain-space units.
// The result is reversible: PanDomain(PanDomain(lo, hi, d), -d) returns
// the original domain. Zero delta returns the domain unchanged.
func PanDomain(lo, hi, delta float64) (float64, float64) {
	return lo + delta, hi + delta
}

// ZoomDomain zooms the domain [lo, hi] around the focal data value by the
// given factor. factor > 1 zooms in (domain shrinks), 0 < factor < 1 zooms
// out (domain expands). The focal data value maps to the same normalized
// position before and after the zoom — this is the focal-point invariance
// property that powers semantic zoom.
//
// Degenerate domains (lo == hi) and non-positive factors return the domain
// unchanged.
func ZoomDomain(lo, hi, focal, factor float64) (float64, float64) {
	if lo == hi || factor <= 0 || math.IsNaN(lo) || math.IsNaN(hi) ||
		math.IsNaN(focal) || math.IsNaN(factor) {
		return lo, hi
	}
	if factor == 1 {
		return lo, hi
	}
	lo = focal - (focal-lo)/factor
	hi = focal + (hi-focal)/factor
	return lo, hi
}
