package runtime

import (
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/diagnostics"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

func TestInspector_spatial_snapshot_shows_resolved_layer_frame(t *testing.T) {
	root := newCoordinateRootFacet(gfx.Translation(15, 25), gfx.RectFromXYWH(0, 0, 200, 200))
	child := newCoordinateHitFacet(gfx.Size{W: 60, H: 40})

	rt := mustRuntimeWithBackend(t, root, &backendFixture{})
	rt.window = &testWindow{width: 400, height: 300}
	rt.AddFacet(root, child, facet.Attachment{LayerID: facet.LayerID(1)})
	rt.RunOneFrame()

	var desc string
	rt.Inspect(func(insp *diagnostics.Inspector) {
		desc = insp.Describe()
		if layer := insp.LayerSnapshots(root.Base().ID()); len(layer) != 1 {
			t.Fatalf("LayerSnapshots = %d, want 1", len(layer))
		} else if !strings.Contains(layer[0].String(), "Frame=") || !strings.Contains(layer[0].String(), "Materialized=") {
			t.Fatalf("layer snapshot = %q", layer[0].String())
		}
	})

	if !strings.Contains(desc, "Frame=LayerID=") || !strings.Contains(desc, "CoordSpace=") || !strings.Contains(desc, "ClipRect=") || !strings.Contains(desc, "Materialized=") {
		t.Fatalf("inspector describe = %q", desc)
	}

	layer, ok := rt.ResolveProjectionLayer(child.Base().ID())
	if !ok {
		t.Fatal("missing resolved projection layer")
	}
	if layer.Transform != gfx.Translation(15, 25) {
		t.Fatalf("resolved transform = %#v", layer.Transform)
	}
	if layer.ClipRect != (gfx.RectFromXYWH(15, 25, 200, 200)) {
		t.Fatalf("resolved clip = %#v", layer.ClipRect)
	}
	if child.Base().LayoutRole() == nil || child.Base().LayoutRole().ArrangedBounds == (gfx.Rect{}) {
		t.Fatal("expected child arranged bounds")
	}
}
