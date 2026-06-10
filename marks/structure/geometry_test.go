package structure

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
)

func TestBoundsAnchorSet(t *testing.T) {
	bounds := gfx.RectFromXYWH(10, 20, 30, 40)
	anchors := boundsAnchorSet(bounds)
	expectBoundsAnchors(t, anchors, bounds)
}

func expectBoundsAnchors(t *testing.T, anchors layout.AnchorSet, bounds gfx.Rect) {
	t.Helper()
	if anchors == nil {
		t.Fatal("expected anchor set")
	}
	want := map[layout.AnchorID]gfx.Point{
		"bounds_center":       rectCenter(bounds),
		"bounds_top_left":     bounds.Min,
		"bounds_top_right":    {X: bounds.Max.X, Y: bounds.Min.Y},
		"bounds_bottom_left":  {X: bounds.Min.X, Y: bounds.Max.Y},
		"bounds_bottom_right": {X: bounds.Max.X, Y: bounds.Max.Y},
	}
	for id, wantPoint := range want {
		got, ok := anchors[id]
		if !ok {
			t.Fatalf("missing anchor %q", id)
		}
		if got != wantPoint {
			t.Fatalf("anchor %q = %#v, want %#v", id, got, wantPoint)
		}
	}
}
