package structure

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks/basic"
)

func TestLayerMount_places_child_in_target_layer(t *testing.T) {
	child := &basic.Rect{
		Bounds: basic.BoundsProps{X: 0, Y: 0, W: 10, H: 10},
		Style:  basic.PrimitiveStyleProps{Visible: true, Opacity: 1},
	}
	root := &LayerMount{
		TargetLayer: 7,
		Child:       child,
	}
	specs := root.OnLayerSpecs()
	if len(specs) != 1 {
		t.Fatalf("LayerSpecs = %d, want 1", len(specs))
	}
	if specs[0].ID != 7 || specs[0].RenderOrder != 7 {
		t.Fatalf("LayerSpec = %#v, want target layer 7", specs[0])
	}
	attachStructureTree(t, root)
	if got := len(root.Base().Children()); got != 1 {
		t.Fatalf("children = %d, want 1", got)
	}
	if root.Base().Children()[0].ID() != child.Base().ID() {
		t.Fatalf("child id = %d, want %d", root.Base().Children()[0].ID(), child.Base().ID())
	}
}

func TestLayerMount_arranges_mounted_child_with_host_bounds(t *testing.T) {
	child := &basic.Rect{
		Bounds: basic.BoundsProps{X: 0, Y: 0, W: 10, H: 10},
		Style:  basic.PrimitiveStyleProps{Visible: true, Opacity: 1},
	}
	root := &LayerMount{
		TargetLayer: 7,
		Child:       child,
	}
	attachStructureTree(t, root)

	hostBounds := gfx.RectFromXYWH(24, 36, 120, 80)
	root.Base().LayoutRole().Arrange(hostBounds)

	if got := child.Base().LayoutRole().ArrangedBounds; got != hostBounds {
		t.Fatalf("child arranged bounds = %#v, want %#v", got, hostBounds)
	}
}

func TestLayerMount_missing_target_reports_diagnostic(t *testing.T) {
	child := &basic.Rect{
		Bounds: basic.BoundsProps{X: 0, Y: 0, W: 10, H: 10},
		Style:  basic.PrimitiveStyleProps{Visible: true, Opacity: 1},
	}
	root := &LayerMount{Child: child}
	expectPanicContains(t, "TargetLayer", func() {
		_ = root.OnLayerSpecs()
	})
}

func TestLayerMount_preserves_child_hit_and_focus(t *testing.T) {
	child := &basic.Rect{
		Bounds: basic.BoundsProps{X: 0, Y: 0, W: 10, H: 10},
		Style:  basic.PrimitiveStyleProps{Visible: true, Opacity: 1},
	}
	if !child.HitTest(gfx.Point{X: 5, Y: 5}) {
		t.Fatal("expected child hit to remain functional")
	}
}
