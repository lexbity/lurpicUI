package facet

import "codeburg.org/lexbit/lurpicui/gfx"

// ScreenToLocal converts a screen-space point to authored local space by
// composing the projection layer's inverse transform with the viewport's
// LayerToLocal. This is the canonical entry point for tooltip, brush, and
// click-to-data hit tests.
//
// The full screen → data chain is:
//
//	data := scale.Invert(ScreenToLocal(layer, viewport, screenPt))
func ScreenToLocal(layer ProjectionLayer, viewport *ViewportRole, screenPt gfx.Point) (gfx.Point, bool) {
	inv, ok := layer.Transform.Inverse()
	if !ok {
		return gfx.Point{}, false
	}
	layerPt := inv.TransformPoint(screenPt)
	return viewport.LayerToLocal(layerPt)
}
