package anchor

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
)

func makeChildNode(anchorRef layout.AnchorID, side layout.AnchorSide, gap float32, size gfx.Size) (layout.ChildNode, *layout.ChildArrangeHandle) {
	handle := &layout.ChildArrangeHandle{}
	node := layout.ChildNode{
		Attachment: layout.ChildAttachment{
			Placement: layout.PlacementHints{
				AnchorRef:  anchorRef,
				AnchorSide: side,
				AnchorGap:  gap,
			},
		},
		IntrinsicSize: size,
	}
	node.AttachArrangeHandle(handle)
	return node, handle
}

func TestPolicyMeasure_returns_zero(t *testing.T) {
	if got := New().Measure(nil, gfx.Size{W: 10, H: 10}); got != (gfx.Size{}) {
		t.Fatalf("Measure() = %#v, want zero", got)
	}
}

func TestPolicyArrange_positions_children_by_side(t *testing.T) {
	cache := layout.NewAnchorPositionCache()
	cache.Update("mark", gfx.Point{X: 100, Y: 200})
	layer := layout.ResolvedLayer{
		Bounds:      gfx.RectFromXYWH(0, 0, 300, 300),
		CoordLimits: layout.CoordLimits{},
		AnchorCache: cache,
	}
	cases := []struct {
		name string
		side layout.AnchorSide
		want gfx.Rect
	}{
		{name: "above", side: layout.AnchorAbove, want: gfx.RectFromXYWH(75, 162, 50, 30)},
		{name: "below", side: layout.AnchorBelow, want: gfx.RectFromXYWH(75, 208, 50, 30)},
		{name: "left", side: layout.AnchorLeft, want: gfx.RectFromXYWH(42, 185, 50, 30)},
		{name: "right", side: layout.AnchorRight, want: gfx.RectFromXYWH(108, 185, 50, 30)},
		{name: "center", side: layout.AnchorCenter, want: gfx.RectFromXYWH(75, 185, 50, 30)},
	}
	p := New()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node, handle := makeChildNode("mark", tc.side, 8, gfx.Size{W: 50, H: 30})
			children := []layout.ChildNode{node}
			handle.Reset()
			p.Arrange(children, layer)
			got, ok := handle.Bounds()
			if !ok {
				t.Fatal("expected arranged bounds")
			}
			if got != tc.want {
				t.Fatalf("bounds = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestPolicyArrange_missing_anchor_uses_zero_rect(t *testing.T) {
	node, handle := makeChildNode("missing", layout.AnchorCenter, 0, gfx.Size{W: 20, H: 10})
	p := New()
	p.Arrange([]layout.ChildNode{node}, layout.ResolvedLayer{
		Bounds:      gfx.RectFromXYWH(0, 0, 100, 100),
		AnchorCache: layout.NewAnchorPositionCache(),
	})
	got, ok := handle.Bounds()
	if !ok {
		t.Fatal("expected arranged bounds")
	}
	if got != (gfx.Rect{}) {
		t.Fatalf("bounds = %#v, want zero rect", got)
	}
}

func TestPolicyArrange_multiple_children_different_anchors(t *testing.T) {
	cache := layout.NewAnchorPositionCache()
	cache.Update("a", gfx.Point{X: 40, Y: 50})
	cache.Update("b", gfx.Point{X: 120, Y: 80})
	first, firstHandle := makeChildNode("a", layout.AnchorRight, 4, gfx.Size{W: 30, H: 10})
	second, secondHandle := makeChildNode("b", layout.AnchorLeft, 6, gfx.Size{W: 20, H: 20})
	p := New()
	p.Arrange([]layout.ChildNode{first, second}, layout.ResolvedLayer{
		Bounds:      gfx.RectFromXYWH(0, 0, 200, 200),
		AnchorCache: cache,
	})
	if got, _ := firstHandle.Bounds(); got != gfx.RectFromXYWH(44, 45, 30, 10) {
		t.Fatalf("first bounds = %#v", got)
	}
	if got, _ := secondHandle.Bounds(); got != gfx.RectFromXYWH(94, 70, 20, 20) {
		t.Fatalf("second bounds = %#v", got)
	}
}

func TestPolicyArrange_clamps_to_bounds(t *testing.T) {
	cache := layout.NewAnchorPositionCache()
	cache.Update("edge", gfx.Point{X: 50, Y: 5})
	node, handle := makeChildNode("edge", layout.AnchorAbove, 10, gfx.Size{W: 20, H: 20})
	p := New()
	p.Arrange([]layout.ChildNode{node}, layout.ResolvedLayer{
		Bounds:      gfx.RectFromXYWH(0, 0, 100, 100),
		AnchorCache: cache,
	})
	if got, _ := handle.Bounds(); got != gfx.RectFromXYWH(40, 0, 20, 20) {
		t.Fatalf("clamped bounds = %#v", got)
	}
}

func TestPolicyArrange_allows_overflow_when_enabled(t *testing.T) {
	cache := layout.NewAnchorPositionCache()
	cache.Update("edge", gfx.Point{X: 50, Y: 5})
	node, handle := makeChildNode("edge", layout.AnchorAbove, 10, gfx.Size{W: 20, H: 20})
	p := New()
	p.Arrange([]layout.ChildNode{node}, layout.ResolvedLayer{
		Bounds:      gfx.RectFromXYWH(0, 0, 100, 100),
		AnchorCache: cache,
		CoordLimits: layout.CoordLimits{AllowOverflow: true},
	})
	if got, _ := handle.Bounds(); got != gfx.RectFromXYWH(40, -25, 20, 20) {
		t.Fatalf("overflow bounds = %#v", got)
	}
}

func TestPolicyArrange_zero_gap_touches_anchor(t *testing.T) {
	cache := layout.NewAnchorPositionCache()
	cache.Update("mark", gfx.Point{X: 100, Y: 200})
	node, handle := makeChildNode("mark", layout.AnchorBelow, 0, gfx.Size{W: 50, H: 30})
	p := New()
	p.Arrange([]layout.ChildNode{node}, layout.ResolvedLayer{
		Bounds:      gfx.RectFromXYWH(0, 0, 300, 300),
		AnchorCache: cache,
	})
	if got, _ := handle.Bounds(); got != gfx.RectFromXYWH(75, 200, 50, 30) {
		t.Fatalf("zero-gap bounds = %#v", got)
	}
}

func BenchmarkPolicyArrange(b *testing.B) {
	cache := layout.NewAnchorPositionCache()
	cache.Update("mark", gfx.Point{X: 100, Y: 200})
	node, handle := makeChildNode("mark", layout.AnchorCenter, 0, gfx.Size{W: 50, H: 30})
	children := []layout.ChildNode{node}
	layer := layout.ResolvedLayer{
		Bounds:      gfx.RectFromXYWH(0, 0, 300, 300),
		AnchorCache: cache,
	}
	p := New()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		handle.Reset()
		p.Arrange(children, layer)
	}
}
