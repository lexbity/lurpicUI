package basic

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/theme"
)

func TestPath_evenodd_fill_hit(t *testing.T) {
	path := &Path{
		Path: gfx.NewPath().
			MoveTo(gfx.Point{X: 0, Y: 0}).
			LineTo(gfx.Point{X: 20, Y: 0}).
			LineTo(gfx.Point{X: 20, Y: 20}).
			LineTo(gfx.Point{X: 0, Y: 20}).
			Close().
			MoveTo(gfx.Point{X: 5, Y: 5}).
			LineTo(gfx.Point{X: 15, Y: 5}).
			LineTo(gfx.Point{X: 15, Y: 15}).
			LineTo(gfx.Point{X: 5, Y: 15}).
			Close().
			Build(),
		FillRule: FillRuleEvenOdd,
		Style: PrimitiveStyleProps{
			Fill: theme.Material{
				Fills:   []theme.Fill{{Type: theme.FillSolid, Color: gfx.Color{R: 1, A: 1}, Opacity: 1}},
				Opacity: 1,
			},
			Visible: true,
			Opacity: 1,
		},
	}
	if !path.HitTest(gfx.Point{X: 2, Y: 2}) {
		t.Fatal("expected outer fill to hit")
	}
	if path.HitTest(gfx.Point{X: 10, Y: 10}) {
		t.Fatal("expected hole to miss under even-odd rule")
	}
}

func TestPath_nonzero_fill_hit(t *testing.T) {
	path := &Path{
		Path: gfx.NewPath().
			MoveTo(gfx.Point{X: 0, Y: 0}).
			LineTo(gfx.Point{X: 20, Y: 0}).
			LineTo(gfx.Point{X: 20, Y: 20}).
			LineTo(gfx.Point{X: 0, Y: 20}).
			Close().
			MoveTo(gfx.Point{X: 5, Y: 5}).
			LineTo(gfx.Point{X: 15, Y: 5}).
			LineTo(gfx.Point{X: 15, Y: 15}).
			LineTo(gfx.Point{X: 5, Y: 15}).
			Close().
			Build(),
		FillRule: FillRuleNonZero,
		Style: PrimitiveStyleProps{
			Fill: theme.Material{
				Fills:   []theme.Fill{{Type: theme.FillSolid, Color: gfx.Color{G: 1, A: 1}, Opacity: 1}},
				Opacity: 1,
			},
			Visible: true,
			Opacity: 1,
		},
	}
	if !path.HitTest(gfx.Point{X: 10, Y: 10}) {
		t.Fatal("expected hole to hit under nonzero rule")
	}
}

func TestPath_stroke_hit_without_fill(t *testing.T) {
	path := &Path{
		Path: gfx.NewPath().
			MoveTo(gfx.Point{X: 0, Y: 0}).
			LineTo(gfx.Point{X: 20, Y: 0}).
			Build(),
		Style: PrimitiveStyleProps{
			Fill: theme.Material{},
			Stroke: theme.MaterialStroke{
				Width: 4,
				Paint: theme.Fill{Type: theme.FillSolid, Color: gfx.Color{B: 1, A: 1}, Opacity: 1},
			},
			Visible: true,
			Opacity: 1,
		},
	}
	if !path.HitTest(gfx.Point{X: 10, Y: 1}) {
		t.Fatal("expected stroked path to hit near the line")
	}
}

func TestPath_bounds_cached_until_geometry_changes(t *testing.T) {
	path := &Path{
		Path: gfx.NewPath().
			MoveTo(gfx.Point{X: 0, Y: 0}).
			LineTo(gfx.Point{X: 10, Y: 0}).
			LineTo(gfx.Point{X: 10, Y: 10}).
			Close().
			Build(),
	}
	_ = path.localBounds()
	if path.boundsRecomputeCount != 1 {
		t.Fatalf("expected one recompute, got %d", path.boundsRecomputeCount)
	}
	_ = path.localBounds()
	if path.boundsRecomputeCount != 1 {
		t.Fatalf("expected cache hit without recompute, got %d", path.boundsRecomputeCount)
	}
	if path.boundsCacheHits != 1 {
		t.Fatalf("expected one cache hit, got %d", path.boundsCacheHits)
	}
	path.Path = gfx.NewPath().
		MoveTo(gfx.Point{X: 0, Y: 0}).
		LineTo(gfx.Point{X: 20, Y: 0}).
		LineTo(gfx.Point{X: 20, Y: 20}).
		Close().
		Build()
	_ = path.localBounds()
	if path.boundsRecomputeCount != 2 {
		t.Fatalf("expected recompute after geometry change, got %d", path.boundsRecomputeCount)
	}
}
