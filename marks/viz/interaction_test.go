package viz

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/scale"
	"codeburg.org/lexbit/lurpicui/scale/reactive"
	"codeburg.org/lexbit/lurpicui/store"
)

func TestScreenToData_round_trip_scatter(t *testing.T) {
	domain := store.NewValueStore([2]float64{0, 100})
	rng := store.NewValueStore([2]float64{0, 300})
	rs := reactive.NewLinearReactive(domain, rng)
	s := rs.Get()

	layer := facet.ProjectionLayer{
		Transform: gfx.Identity(),
	}
	viewport := &facet.ViewportRole{
		Transform: gfx.Identity(),
	}

	// For a data value of 50, Map(50) = 150
	pixel := s.Map(50)

	screenPt := gfx.Point{X: float32(pixel), Y: 100}

	inv, ok := ScreenToData(layer, viewport, screenPt, s)
	if !ok {
		t.Fatal("ScreenToData returned false")
	}
	if inv != 50 {
		t.Fatalf("ScreenToData = %f, want 50", inv)
	}
}

func TestScreenToData_with_pan_zoom(t *testing.T) {
	domain := store.NewValueStore([2]float64{0, 100})
	rng := store.NewValueStore([2]float64{0, 300})
	rs := reactive.NewLinearReactive(domain, rng)
	s := rs.Get()

	layer := facet.ProjectionLayer{
		Transform: gfx.Identity(),
	}
	// Pan +50px, zoom 2x: Transform = {A:2, D:2, TX:50, TY:0}
	// Inverse = {A:0.5, D:0.5, TX:-25, TY:0}
	// LayerToLocal(layerX) = (layerX - 50) / 2
	viewport := &facet.ViewportRole{}
	viewport.SetPanZoom(gfx.Point{X: 50, Y: 0}, 2)

	// Data value 50 → Map(50) = 150 in layer space
	// To find screen position where this point appears:
	// LayerToLocal(150) = (150 - 50)/2 = 50 (local)
	// With identity layer, screen = layer = 150
	// So screen 150 → local 50 → Invert(50) = 16.666...
	// Instead, find screen such that local = 150 (Invert(150)=50):
	// (layer - 50) / 2 = 150 → layer = 350
	screenPt := gfx.Point{X: 350, Y: 100}

	inv, ok := ScreenToData(layer, viewport, screenPt, s)
	if !ok {
		t.Fatal("ScreenToData returned false")
	}
	if inv != 50 {
		t.Fatalf("ScreenToData = %f, want 50", inv)
	}
}

func TestScreenToDataY_round_trip(t *testing.T) {
	domain := store.NewValueStore([2]float64{0, 100})
	rng := store.NewValueStore([2]float64{0, 300})
	rs := reactive.NewLinearReactive(domain, rng)
	s := rs.Get()

	layer := facet.ProjectionLayer{
		Transform: gfx.Identity(),
	}
	viewport := &facet.ViewportRole{
		Transform: gfx.Identity(),
	}

	pixel := s.Map(75)
	screenPt := gfx.Point{X: 0, Y: float32(pixel)}

	inv, ok := ScreenToDataY(layer, viewport, screenPt, s)
	if !ok {
		t.Fatal("ScreenToDataY returned false")
	}
	if inv != 75 {
		t.Fatalf("ScreenToDataY = %f, want 75", inv)
	}
}

func TestScreenToCategory_band_scale(t *testing.T) {
	band := scale.NewBand([]string{"A", "B", "C"},
		scale.WithRange(0, 300),
	)

	layer := facet.ProjectionLayer{
		Transform: gfx.Identity(),
	}
	viewport := &facet.ViewportRole{
		Transform: gfx.Identity(),
	}

	// Band B should be at approximately step position
	// n=3, span=300, padding=0 → step=100, bandwidth=100
	// Band A: [0, 100), Band B: [100, 200), Band C: [200, 300)
	startB, _, ok := band.Band("B")
	if !ok {
		t.Fatal("expected band B to exist")
	}
	screenPt := gfx.Point{X: float32(startB + 10), Y: 0}

	cat, hit := ScreenToCategory(layer, viewport, screenPt, band)
	if !hit {
		t.Fatal("ScreenToCategory returned false for band B")
	}
	if cat != "B" {
		t.Fatalf("ScreenToCategory = %q, want %q", cat, "B")
	}
}

func TestScreenToCategory_gap_miss(t *testing.T) {
	band := scale.NewBand([]string{"A", "B"},
		scale.WithRange(0, 200),
		scale.WithPaddingInner(0.5),
	)

	layer := facet.ProjectionLayer{
		Transform: gfx.Identity(),
	}
	viewport := &facet.ViewportRole{
		Transform: gfx.Identity(),
	}

	// With 2 bands, range=200, padding=0.5:
	// step=200/(2-0.5)=133.33, bandwidth=66.67, start=0
	// Band A: [0, 66.67), gap: [66.67, 133.33), Band B: [133.33, 200)
	gapPt := gfx.Point{X: 100, Y: 0}

	_, hit := ScreenToCategory(layer, viewport, gapPt, band)
	if hit {
		t.Fatal("expected miss in the gap between bands")
	}
}

func TestScreenToCategory_with_pan_zoom(t *testing.T) {
	band := scale.NewBand([]string{"X", "Y", "Z"},
		scale.WithRange(0, 300),
	)

	layer := facet.ProjectionLayer{
		Transform: gfx.Identity(),
	}
	// Zoom 2x, pan +0, so local*2 = screen → local = screen/2
	viewport := &facet.ViewportRole{}
	viewport.SetPanZoom(gfx.Point{X: 0, Y: 0}, 2)

	// Band Y: local [100, 200), so screen = [200, 400)
	startY, _, _ := band.Band("Y")
	screenPt := gfx.Point{X: float32(startY*2 + 10), Y: 0}

	cat, hit := ScreenToCategory(layer, viewport, screenPt, band)
	if !hit {
		t.Fatal("expected hit on band Y with zoom")
	}
	if cat != "Y" {
		t.Fatalf("ScreenToCategory with zoom = %q, want %q", cat, "Y")
	}
}

func TestScreenToData_non_invertible_layer(t *testing.T) {
	// A transform that cannot be inverted (zero scale)
	layer := facet.ProjectionLayer{
		Transform: gfx.Transform{}, // all zeros — singular
	}
	viewport := &facet.ViewportRole{
		Transform: gfx.Identity(),
	}
	s := scale.NewLinear()

	_, ok := ScreenToData(layer, viewport, gfx.Point{X: 50, Y: 50}, s)
	if ok {
		t.Fatal("expected false for non-invertible layer")
	}
}
