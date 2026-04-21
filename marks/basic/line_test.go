package basic

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/theme"
)

func TestLine_hit_uses_stroke_width(t *testing.T) {
	line := &Line{
		Start:  gfx.Point{X: 0, Y: 0},
		End:    gfx.Point{X: 10, Y: 0},
		Stroke: theme.MaterialStroke{Width: 4, Paint: theme.Fill{Type: theme.FillSolid, Color: gfx.Color{A: 1}, Opacity: 1}},
	}
	if !line.HitTest(gfx.Point{X: 5, Y: 1}) {
		t.Fatal("expected near-line point to hit")
	}
	if line.HitTest(gfx.Point{X: 5, Y: 3}) {
		t.Fatal("expected far point to miss")
	}
}

func TestLine_endpoints_exported_as_anchors(t *testing.T) {
	line := &Line{
		Start:  gfx.Point{X: 10, Y: 20},
		End:    gfx.Point{X: 30, Y: 60},
		Stroke: theme.MaterialStroke{Width: 4, Paint: theme.Fill{Type: theme.FillSolid, Color: gfx.Color{A: 1}, Opacity: 1}},
	}
	anchors := line.ExportAnchors(layout.AnchorExportContext{})
	for _, name := range []layout.AnchorID{"start", "mid", "end"} {
		if _, ok := anchors[name]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}
}
