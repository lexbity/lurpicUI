package data

import (
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/store"
)

// RegionFromBounds pushes resolved layout bounds into a range store.
// The range is set to [0, width] so that reactive scales map domain values
// to pixel positions within the plot region. Call this from arrange after
// the plot bounds are known.
func RegionFromBounds(rng *store.ValueStore[[2]float64], bounds gfx.Rect) {
	if rng == nil {
		return
	}
	rng.Set([2]float64{0, float64(bounds.Width())})
}
