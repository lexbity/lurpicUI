package viz

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/scale"
)

// ScreenToData converts screen-space coordinates through a projection layer
// and viewport transform, then inverts through the given scale to recover a
// domain value. Returns (value, true) on success.
//
// This is the canonical entry point for tooltip and click-to-data interactions
// on continuous scales (linear, log, time).
func ScreenToData(
	layer facet.ProjectionLayer,
	viewport *facet.ViewportRole,
	screenPt gfx.Point,
	s scale.InvertibleScale,
) (float64, bool) {
	localPt, ok := facet.ScreenToLocal(layer, viewport, screenPt)
	if !ok {
		return 0, false
	}
	return s.Invert(float64(localPt.X)), true
}

// ScreenToCategory converts screen-space coordinates through a projection
// layer and viewport transform, then inverts through the given band scale to
// recover a category name. Returns ("", false) if no band is hit.
//
// This is the canonical entry point for tooltip and click-to-data interactions
// on ordinal/band scales.
func ScreenToCategory(
	layer facet.ProjectionLayer,
	viewport *facet.ViewportRole,
	screenPt gfx.Point,
	band scale.BandScale,
) (string, bool) {
	localPt, ok := facet.ScreenToLocal(layer, viewport, screenPt)
	if !ok {
		return "", false
	}
	return band.InvertRange(float64(localPt.X))
}

// ScreenToDataY is like ScreenToData but inverts through the Y scale.
func ScreenToDataY(
	layer facet.ProjectionLayer,
	viewport *facet.ViewportRole,
	screenPt gfx.Point,
	s scale.InvertibleScale,
) (float64, bool) {
	localPt, ok := facet.ScreenToLocal(layer, viewport, screenPt)
	if !ok {
		return 0, false
	}
	return s.Invert(float64(localPt.Y)), true
}
