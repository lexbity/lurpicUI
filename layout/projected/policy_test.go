package projected

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
)

func makeProjectedChild(worldPos gfx.Point, worldSize gfx.Size) (layout.ChildNode, *layout.ChildArrangeHandle) {
	handle := &layout.ChildArrangeHandle{}
	node := layout.ChildNode{
		WorldPosition: worldPos,
		WorldSize:     worldSize,
		HasWorldSpace: true,
	}
	node.AttachArrangeHandle(handle)
	return node, handle
}

func TestPolicyMeasure_returns_zero(t *testing.T) {
	if got := New().Measure(nil, gfx.Size{W: 1, H: 1}); got != (gfx.Size{}) {
		t.Fatalf("Measure() = %#v, want zero", got)
	}
}

func TestPolicyArrange_identity_transform(t *testing.T) {
	node, handle := makeProjectedChild(gfx.Point{X: 100, Y: 200}, gfx.Size{W: 50, H: 30})
	New().Arrange([]layout.ChildNode{node}, layout.ResolvedLayer{Transform: gfx.Identity()})
	if got, _ := handle.Bounds(); got != gfx.RectFromXYWH(100, 200, 50, 30) {
		t.Fatalf("bounds = %#v", got)
	}
}

func TestPolicyArrange_scale_transform(t *testing.T) {
	node, handle := makeProjectedChild(gfx.Point{X: 100, Y: 200}, gfx.Size{W: 50, H: 30})
	New().Arrange([]layout.ChildNode{node}, layout.ResolvedLayer{Transform: gfx.Scale(2, 2)})
	if got, _ := handle.Bounds(); got != gfx.RectFromXYWH(200, 400, 100, 60) {
		t.Fatalf("bounds = %#v", got)
	}
}

func TestPolicyArrange_translation_transform(t *testing.T) {
	node, handle := makeProjectedChild(gfx.Point{X: 100, Y: 200}, gfx.Size{W: 50, H: 30})
	New().Arrange([]layout.ChildNode{node}, layout.ResolvedLayer{Transform: gfx.Translation(-50, -50)})
	if got, _ := handle.Bounds(); got != gfx.RectFromXYWH(50, 150, 50, 30) {
		t.Fatalf("bounds = %#v", got)
	}
}

func TestPolicyArrange_combined_transform(t *testing.T) {
	node, handle := makeProjectedChild(gfx.Point{X: 100, Y: 200}, gfx.Size{W: 50, H: 30})
	New().Arrange([]layout.ChildNode{node}, layout.ResolvedLayer{Transform: gfx.Translation(-50, -50).Multiply(gfx.Scale(2, 2))})
	if got, _ := handle.Bounds(); got != gfx.RectFromXYWH(150, 350, 100, 60) {
		t.Fatalf("bounds = %#v", got)
	}
}

func TestPolicyArrange_missing_world_space_zeroes_rect(t *testing.T) {
	handle := &layout.ChildArrangeHandle{}
	node := layout.ChildNode{}
	node.AttachArrangeHandle(handle)
	New().Arrange([]layout.ChildNode{node}, layout.ResolvedLayer{})
	if got, _ := handle.Bounds(); got != (gfx.Rect{}) {
		t.Fatalf("bounds = %#v, want zero", got)
	}
}
