package basic

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/theme"
)

func TestPolygon_fill_hit(t *testing.T) {
	poly := &Polygon{
		Points: []gfx.Point{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 10, Y: 10}, {X: 0, Y: 10}},
		Style: PrimitiveStyleProps{
			Fill: theme.Material{
				Fills:   []theme.Fill{{Type: theme.FillSolid, Color: gfx.Color{R: 1, A: 1}, Opacity: 1}},
				Opacity: 1,
			},
			Visible: true,
			Opacity: 1,
		},
	}
	if !poly.HitTest(gfx.Point{X: 5, Y: 5}) {
		t.Fatal("expected inside polygon hit")
	}
	if poly.HitTest(gfx.Point{X: 15, Y: 5}) {
		t.Fatal("expected outside polygon miss")
	}
}

func TestPolygon_centroid_anchor_present_for_valid_polygon(t *testing.T) {
	poly := &Polygon{
		Points: []gfx.Point{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 10, Y: 10}, {X: 0, Y: 10}},
	}
	anchors := poly.ExportAnchors(layout.AnchorExportContext{})
	if _, ok := anchors["centroid"]; !ok {
		t.Fatal("expected centroid anchor")
	}
}
