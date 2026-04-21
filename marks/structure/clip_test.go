package structure

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
)

func TestClip_descendants_render_clipped(t *testing.T) {
	child := newTestShapeFacet(gfx.RectFromXYWH(40, 10, 30, 30))
	root := &Clip{
		Bounds:   gfx.RectFromXYWH(0, 0, 50, 50),
		Children: []marks.Mark{child},
	}
	rt, backend := newStructureRuntime(t, root, 100, 100)
	rt.RunOneFrame()
	layer, ok := rt.ResolveProjectionLayer(child.Base().ID())
	if !ok {
		t.Fatal("missing child projection layer")
	}
	want := gfx.RectFromXYWH(0, 0, 50, 50)
	if layer.ClipRect != want {
		t.Fatalf("ClipRect = %#v, want %#v", layer.ClipRect, want)
	}
	if backend.last == nil || len(backend.last.Layers) == 0 {
		t.Fatal("missing recorded frame layers")
	}
	if backend.last.Layers[0].ClipRect != want {
		t.Fatalf("frame clip = %#v, want %#v", backend.last.Layers[0].ClipRect, want)
	}
}

func TestClip_descendants_hit_clipped(t *testing.T) {
	child := newTestShapeFacet(gfx.RectFromXYWH(40, 10, 30, 30))
	root := &Clip{
		Bounds:   gfx.RectFromXYWH(0, 0, 50, 50),
		Children: []marks.Mark{child},
	}
	rt, _ := newStructureRuntime(t, root, 100, 100)
	rt.RunOneFrame()
	if got := rt.HitTest(gfx.Point{X: 60, Y: 20}); got != 0 {
		t.Fatalf("HitTest outside clip = %d, want 0", got)
	}
	if got := rt.HitTest(gfx.Point{X: 45, Y: 20}); got != child.Base().ID() {
		t.Fatalf("HitTest inside clip = %d, want %d", got, child.Base().ID())
	}
}

func TestClip_exports_clip_bounds_anchors(t *testing.T) {
	root := &Clip{Bounds: gfx.RectFromXYWH(0, 0, 50, 50)}
	anchors := root.ExportAnchors(layout.AnchorExportContext{})
	for _, name := range []string{"bounds-center", "top-left", "bottom-right"} {
		if _, ok := anchors[layout.AnchorID(name)]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}
}
