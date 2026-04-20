package free

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
)

func newNode(anchor layout.FreeAnchor, offset gfx.Point) (layout.ChildNode, *layout.ChildArrangeHandle) {
	handle := &layout.ChildArrangeHandle{}
	node := layout.ChildNode{
		FacetID: facet.FacetID(1),
		Attachment: layout.ChildAttachment{
			Placement: layout.PlacementHints{
				FreeAnchor: anchor,
				Offset:     offset,
			},
		},
		IntrinsicSize: gfx.Size{W: 20, H: 10},
	}
	node.AttachArrangeHandle(handle)
	return node, handle
}

func TestMeasureReturnsZero(t *testing.T) {
	p := New()
	if got := p.Measure([]layout.ChildNode{{}}, gfx.Size{W: 100, H: 100}); got != (gfx.Size{}) {
		t.Fatalf("unexpected size: %#v", got)
	}
}

func TestAllAnchors(t *testing.T) {
	cases := []struct {
		name   string
		anchor layout.FreeAnchor
		want   gfx.Rect
	}{
		{name: "top-left", anchor: layout.FreeTopLeft, want: gfx.RectFromXYWH(10, 20, 20, 10)},
		{name: "top-center", anchor: layout.FreeTopCenter, want: gfx.RectFromXYWH(50, 20, 20, 10)},
		{name: "top-right", anchor: layout.FreeTopRight, want: gfx.RectFromXYWH(90, 20, 20, 10)},
		{name: "center-left", anchor: layout.FreeCenterLeft, want: gfx.RectFromXYWH(10, 65, 20, 10)},
		{name: "center", anchor: layout.FreeCenter, want: gfx.RectFromXYWH(50, 65, 20, 10)},
		{name: "center-right", anchor: layout.FreeCenterRight, want: gfx.RectFromXYWH(90, 65, 20, 10)},
		{name: "bottom-left", anchor: layout.FreeBottomLeft, want: gfx.RectFromXYWH(10, 110, 20, 10)},
		{name: "bottom-center", anchor: layout.FreeBottomCenter, want: gfx.RectFromXYWH(50, 110, 20, 10)},
		{name: "bottom-right", anchor: layout.FreeBottomRight, want: gfx.RectFromXYWH(90, 110, 20, 10)},
	}
	p := New()
	for _, tc := range cases {
		child, handle := newNode(tc.anchor, gfx.Point{})
		p.Arrange([]layout.ChildNode{child}, layout.ResolvedLayer{Bounds: gfx.RectFromXYWH(10, 20, 100, 100)})
		got, ok := handle.Bounds()
		if !ok || got != tc.want {
			t.Fatalf("%s: unexpected bounds %#v", tc.name, got)
		}
	}
}

func TestOffsetApplication(t *testing.T) {
	p := New()
	child, handle := newNode(layout.FreeTopLeft, gfx.Point{X: 5, Y: 7})
	p.Arrange([]layout.ChildNode{child}, layout.ResolvedLayer{Bounds: gfx.RectFromXYWH(0, 0, 100, 100)})
	got, ok := handle.Bounds()
	if !ok || got.Min != (gfx.Point{X: 5, Y: 7}) {
		t.Fatalf("unexpected offset result %#v", got)
	}
}

func TestBottomRightNegativeOffset(t *testing.T) {
	p := New()
	child, handle := newNode(layout.FreeBottomRight, gfx.Point{X: -8, Y: -8})
	p.Arrange([]layout.ChildNode{child}, layout.ResolvedLayer{Bounds: gfx.RectFromXYWH(0, 0, 100, 100)})
	got, ok := handle.Bounds()
	if !ok || got.Max != (gfx.Point{X: 92, Y: 92}) {
		t.Fatalf("unexpected inset bounds %#v", got)
	}
}

func TestClampToBounds(t *testing.T) {
	p := New()
	child, handle := newNode(layout.FreeTopLeft, gfx.Point{X: -20, Y: -20})
	p.Arrange([]layout.ChildNode{child}, layout.ResolvedLayer{Bounds: gfx.RectFromXYWH(0, 0, 50, 50)})
	got, ok := handle.Bounds()
	if !ok || got.Min != (gfx.Point{}) {
		t.Fatalf("unexpected clamp bounds %#v", got)
	}
}

func TestAllowOverflow_skips_clamp(t *testing.T) {
	p := New()
	child, handle := newNode(layout.FreeTopLeft, gfx.Point{X: -20, Y: -20})
	p.Arrange([]layout.ChildNode{child}, layout.ResolvedLayer{
		Bounds:      gfx.RectFromXYWH(0, 0, 50, 50),
		CoordLimits: layout.CoordLimits{AllowOverflow: true},
	})
	got, ok := handle.Bounds()
	if !ok || got.Min != (gfx.Point{X: -20, Y: -20}) {
		t.Fatalf("unexpected overflow bounds %#v", got)
	}
}

func TestMultipleChildrenDifferentAnchors(t *testing.T) {
	p := New()
	left, leftHandle := newNode(layout.FreeTopLeft, gfx.Point{})
	right, rightHandle := newNode(layout.FreeBottomRight, gfx.Point{})
	p.Arrange([]layout.ChildNode{left, right}, layout.ResolvedLayer{Bounds: gfx.RectFromXYWH(0, 0, 100, 100)})

	leftBounds, ok := leftHandle.Bounds()
	if !ok || leftBounds.Min != (gfx.Point{X: 0, Y: 0}) {
		t.Fatalf("unexpected left bounds %#v", leftBounds)
	}

	rightBounds, ok := rightHandle.Bounds()
	if !ok || rightBounds.Max != (gfx.Point{X: 100, Y: 100}) {
		t.Fatalf("unexpected right bounds %#v", rightBounds)
	}
}

func TestMultipleChildrenSameAnchor(t *testing.T) {
	p := New()
	first, firstHandle := newNode(layout.FreeCenter, gfx.Point{})
	second, secondHandle := newNode(layout.FreeCenter, gfx.Point{})
	p.Arrange([]layout.ChildNode{first, second}, layout.ResolvedLayer{Bounds: gfx.RectFromXYWH(0, 0, 100, 100)})

	firstBounds, ok := firstHandle.Bounds()
	if !ok || firstBounds != (gfx.RectFromXYWH(40, 45, 20, 10)) {
		t.Fatalf("unexpected first bounds %#v", firstBounds)
	}

	secondBounds, ok := secondHandle.Bounds()
	if !ok || secondBounds != firstBounds {
		t.Fatalf("unexpected second bounds %#v", secondBounds)
	}
}

func BenchmarkPolicyArrange(b *testing.B) {
	p := New()
	children := make([]layout.ChildNode, 3)
	handles := make([]layout.ChildArrangeHandle, 3)
	anchors := []layout.FreeAnchor{
		layout.FreeTopLeft,
		layout.FreeCenter,
		layout.FreeBottomRight,
	}
	for i := range children {
		children[i] = layout.ChildNode{
			FacetID: facet.FacetID(i + 1),
			Attachment: layout.ChildAttachment{
				Placement: layout.PlacementHints{
					FreeAnchor: anchors[i],
				},
			},
			IntrinsicSize: gfx.Size{W: 20, H: 10},
		}
		children[i].AttachArrangeHandle(&handles[i])
	}

	layer := layout.ResolvedLayer{Bounds: gfx.RectFromXYWH(0, 0, 100, 100)}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for j := range handles {
			handles[j] = layout.ChildArrangeHandle{}
		}
		p.Arrange(children, layer)
	}
}
