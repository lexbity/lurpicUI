package facet

import (
	"math"
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
)

func almostEqualPoint(a, b gfx.Point) bool {
	const eps = 1e-5
	return float64(math.Abs(float64(a.X-b.X))) <= eps &&
		float64(math.Abs(float64(a.Y-b.Y))) <= eps
}

func TestScreenToLocal_identity_transform(t *testing.T) {
	layer := ProjectionLayer{
		Transform: gfx.Identity(),
	}
	viewport := &ViewportRole{
		Transform: gfx.Identity(),
	}
	screenPt := gfx.Point{X: 100, Y: 200}

	localPt, ok := ScreenToLocal(layer, viewport, screenPt)
	if !ok {
		t.Fatal("expected ok=true for identity transform")
	}
	if !almostEqualPoint(localPt, screenPt) {
		t.Fatalf("local = %+v, want %+v", localPt, screenPt)
	}
}

func TestScreenToLocal_round_trip(t *testing.T) {
	layer := ProjectionLayer{
		Transform: gfx.Translation(50, 100),
	}
	viewport := &ViewportRole{
		Transform: gfx.Scale(2, 3),
	}

	// Start with a local point and compute its screen position
	localOrig := gfx.Point{X: 30, Y: 40}
	layerPt := viewport.LocalToLayer(localOrig)
	screenPt := layer.Transform.TransformPoint(layerPt)

	// Now convert back
	localGot, ok := ScreenToLocal(layer, viewport, screenPt)
	if !ok {
		t.Fatal("expected ok=true for round-trip")
	}
	if !almostEqualPoint(localGot, localOrig) {
		t.Fatalf("round-trip: got %+v, want %+v", localGot, localOrig)
	}
}

func TestScreenToLocal_with_pan_zoom(t *testing.T) {
	layer := ProjectionLayer{
		Transform: gfx.Identity(),
	}
	viewport := &ViewportRole{}
	viewport.SetPanZoom(gfx.Point{X: 50, Y: 100}, 2)

	// A local point
	localOrig := gfx.Point{X: 30, Y: 40}
	layerPt := viewport.LocalToLayer(localOrig)
	// With layer transform = identity, screen = layer
	screenPt := layer.Transform.TransformPoint(layerPt)

	// Convert back
	localGot, ok := ScreenToLocal(layer, viewport, screenPt)
	if !ok {
		t.Fatal("expected ok=true for pan/zoom")
	}
	if !almostEqualPoint(localGot, localOrig) {
		t.Fatalf("pan/zoom: got %+v, want %+v", localGot, localOrig)
	}
}

func TestScreenToLocal_non_invertible_layer(t *testing.T) {
	// A zero transform is not invertible
	layer := ProjectionLayer{
		Transform: gfx.Transform{}, // all zeros → singular
	}
	viewport := &ViewportRole{
		Transform: gfx.Identity(),
	}
	_, ok := ScreenToLocal(layer, viewport, gfx.Point{X: 1, Y: 1})
	if ok {
		t.Fatal("expected ok=false for non-invertible layer transform")
	}
}

func TestScreenToLocal_nil_viewport(t *testing.T) {
	layer := ProjectionLayer{
		Transform: gfx.Identity(),
	}
	_, ok := ScreenToLocal(layer, nil, gfx.Point{X: 1, Y: 1})
	if ok {
		t.Fatal("expected ok=false for nil viewport")
	}
}

func TestScreenToLocal_non_invertible_viewport(t *testing.T) {
	layer := ProjectionLayer{
		Transform: gfx.Identity(),
	}
	// A zero viewport transform is not invertible
	viewport := &ViewportRole{
		Transform: gfx.Transform{}, // all zeros → singular
	}
	_, ok := ScreenToLocal(layer, viewport, gfx.Point{X: 1, Y: 1})
	if ok {
		t.Fatal("expected ok=false for non-invertible viewport")
	}
}

func TestScreenToLocal_complex_transform_chain(t *testing.T) {
	// Layer with translation, viewport with scale
	layer := ProjectionLayer{
		Transform: gfx.Translation(100, 200),
	}
	viewport := &ViewportRole{}
	viewport.SetPanZoom(gfx.Point{X: 10, Y: 20}, 3)

	localOrig := gfx.Point{X: 5, Y: 15}
	layerPt := viewport.LocalToLayer(localOrig)
	screenPt := layer.Transform.TransformPoint(layerPt)

	localGot, ok := ScreenToLocal(layer, viewport, screenPt)
	if !ok {
		t.Fatal("expected ok=true for complex chain")
	}
	if !almostEqualPoint(localGot, localOrig) {
		t.Fatalf("complex: got %+v, want %+v", localGot, localOrig)
	}
}

func TestScreenToLocal_zero_point(t *testing.T) {
	layer := ProjectionLayer{
		Transform: gfx.Translation(50, 100),
	}
	viewport := &ViewportRole{
		Transform: gfx.Scale(2, 2),
	}

	localOrig := gfx.Point{X: 0, Y: 0}
	layerPt := viewport.LocalToLayer(localOrig)
	screenPt := layer.Transform.TransformPoint(layerPt)

	localGot, ok := ScreenToLocal(layer, viewport, screenPt)
	if !ok {
		t.Fatal("expected ok=true for zero point")
	}
	if !almostEqualPoint(localGot, localOrig) {
		t.Fatalf("zero point: got %+v, want %+v", localGot, localOrig)
	}
}
