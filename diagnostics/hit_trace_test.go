package diagnostics

import (
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
)

func TestHitTestTrace_String_includes_resolved_layer_frame(t *testing.T) {
	trace := HitTestTrace{
		Result: 42,
		TestedLayers: []LayerHitTrace{{
			ParentID:    1,
			LayerID:     2,
			CoordSpace:  layout.CoordViewport,
			RenderOrder: 7,
			HitPolicy:   layout.HitPassThrough,
			Bounds:      gfx.RectFromXYWH(10, 20, 30, 40),
			ClipRect:    gfx.RectFromXYWH(10, 20, 30, 40),
			Transform:   gfx.Translation(15, 25),
			TestedCount: 3,
			HitFacetID:  facet.FacetID(42),
			StoppedHere: true,
		}},
	}

	s := trace.String()
	if !strings.Contains(s, "CoordSpace=") || !strings.Contains(s, "ClipRect=") || !strings.Contains(s, "Transform=") {
		t.Fatalf("trace string = %q", s)
	}
}
