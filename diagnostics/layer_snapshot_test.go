package diagnostics

import (
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
)

func TestLayerSnapshot_Frame_exposes_resolved_spatial_contract(t *testing.T) {
	snap := LayerSnapshot{
		LayerID:        7,
		LayerName:      "test",
		WindowBinding:  "primary",
		CoordSpace:     layout.CoordViewport,
		Bounds:         gfx.RectFromXYWH(10, 20, 30, 40),
		ClipRect:       gfx.RectFromXYWH(10, 20, 30, 40),
		Transform:      gfx.Translation(15, 25),
		RenderOrder:    9,
		HitPolicy:      layout.HitPassThrough,
		RootPolicyKind: "grid",
		Materialized:   true,
		CommandCount:   3,
		HitRegionCount: 2,
	}

	frame := snap.Frame()
	if frame.LayerID != 7 || frame.CoordSpace != layout.CoordViewport {
		t.Fatalf("frame = %#v", frame)
	}
	if frame.Bounds != (gfx.RectFromXYWH(10, 20, 30, 40)) {
		t.Fatalf("frame bounds = %#v", frame.Bounds)
	}
	if frame.Transform != gfx.Translation(15, 25) {
		t.Fatalf("frame transform = %#v", frame.Transform)
	}
	if frame.ClipRect != (gfx.RectFromXYWH(10, 20, 30, 40)) {
		t.Fatalf("frame clip = %#v", frame.ClipRect)
	}

	s := snap.String()
	if !strings.Contains(s, "Frame=") || !strings.Contains(s, "CoordSpace=") || !strings.Contains(s, "ClipRect=") || !strings.Contains(s, "Materialized=true") {
		t.Fatalf("snapshot string = %q", s)
	}
}
