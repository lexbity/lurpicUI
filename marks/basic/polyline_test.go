package basic

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/theme"
)

func TestPolyline_hit_near_segment(t *testing.T) {
	line := &Polyline{
		Points: []gfx.Point{{0, 0}, {10, 0}},
		Stroke: theme.MaterialStroke{Width: 4, Paint: theme.Fill{Type: theme.FillSolid, Color: gfx.Color{A: 1}, Opacity: 1}},
	}
	if !line.HitTest(gfx.Point{X: 4, Y: 1}) {
		t.Fatal("expected near segment hit")
	}
	if line.HitTest(gfx.Point{X: 4, Y: 4}) {
		t.Fatal("expected far point miss")
	}
}

func TestPolyline_exports_start_end_anchors(t *testing.T) {
	line := &Polyline{
		Points: []gfx.Point{{10, 20}, {30, 40}},
		Stroke: theme.MaterialStroke{Width: 2, Paint: theme.Fill{Type: theme.FillSolid, Color: gfx.Color{A: 1}, Opacity: 1}},
	}
	anchors := line.ExportAnchors(layout.AnchorExportContext{})
	for _, name := range []layout.AnchorID{"start", "end"} {
		if _, ok := anchors[name]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}
}
