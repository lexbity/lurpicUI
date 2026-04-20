package split

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
)

func newNode(id facet.FacetID, intrinsic, min gfx.Size, flex float32, offset gfx.Point) (layout.ChildNode, *layout.ChildArrangeHandle) {
	handle := &layout.ChildArrangeHandle{}
	node := layout.ChildNode{
		FacetID: id,
		Attachment: layout.ChildAttachment{
			Placement: layout.PlacementHints{
				Flex:   flex,
				Offset: offset,
			},
		},
		IntrinsicSize: intrinsic,
		MinSize:       min,
	}
	node.AttachArrangeHandle(handle)
	return node, handle
}

func TestHorizontalSplitStructural_intrinsic_panes(t *testing.T) {
	p := New(Config{Axis: Horizontal})
	a, _ := newNode(1, gfx.Size{W: 100, H: 20}, gfx.Size{}, 0, gfx.Point{})
	b, _ := newNode(2, gfx.Size{W: 60, H: 30}, gfx.Size{}, 0, gfx.Point{})

	got := p.Measure([]layout.ChildNode{a, b}, gfx.Size{W: 400, H: 100})
	if got != (gfx.Size{W: 160, H: 30}) {
		t.Fatalf("unexpected size: %#v", got)
	}
}

func TestHorizontalSplitStructural_fixed_and_weighted(t *testing.T) {
	p := New(Config{Axis: Horizontal})
	a, ha := newNode(1, gfx.Size{W: 100, H: 20}, gfx.Size{}, 0, gfx.Point{X: 200})
	b, hb := newNode(2, gfx.Size{W: 60, H: 30}, gfx.Size{}, 1, gfx.Point{})

	p.Arrange([]layout.ChildNode{a, b}, layout.ResolvedLayer{Bounds: gfx.RectFromXYWH(0, 0, 400, 100)})

	if got, ok := ha.Bounds(); !ok || !almostEqual(got.Width(), 200) {
		t.Fatalf("unexpected first width: %#v", got)
	}
	if got, ok := hb.Bounds(); !ok || !almostEqual(got.Width(), 200) {
		t.Fatalf("unexpected second width: %#v", got)
	}
}

func TestHorizontalSplitStructural_twoWeighted_ratio(t *testing.T) {
	p := New(Config{Axis: Horizontal})
	a, ha := newNode(1, gfx.Size{W: 10, H: 20}, gfx.Size{}, 1, gfx.Point{})
	b, hb := newNode(2, gfx.Size{W: 10, H: 20}, gfx.Size{}, 2, gfx.Point{})

	p.Arrange([]layout.ChildNode{a, b}, layout.ResolvedLayer{Bounds: gfx.RectFromXYWH(0, 0, 300, 100)})

	if got, ok := ha.Bounds(); !ok || !almostEqual(got.Width(), 100) {
		t.Fatalf("unexpected first width: %#v", got)
	}
	if got, ok := hb.Bounds(); !ok || !almostEqual(got.Width(), 200) {
		t.Fatalf("unexpected second width: %#v", got)
	}
}

func TestDividerSize_deducted_between_panes(t *testing.T) {
	p := New(Config{Axis: Horizontal, DividerSize: 4})
	a, ha := newNode(1, gfx.Size{W: 10, H: 20}, gfx.Size{}, 0, gfx.Point{X: 100})
	b, hb := newNode(2, gfx.Size{W: 10, H: 20}, gfx.Size{}, 0, gfx.Point{X: 100})

	p.Arrange([]layout.ChildNode{a, b}, layout.ResolvedLayer{Bounds: gfx.RectFromXYWH(0, 0, 400, 100)})

	if got, ok := ha.Bounds(); !ok || got.Width() != 100 {
		t.Fatalf("unexpected first width: %#v", got)
	}
	if got, ok := hb.Bounds(); !ok || got.Min.X != 104 {
		t.Fatalf("unexpected second origin: %#v", got)
	}
}

func TestMinSize_enforced_for_weighted(t *testing.T) {
	p := New(Config{Axis: Horizontal})
	a, ha := newNode(1, gfx.Size{W: 10, H: 20}, gfx.Size{W: 120, H: 20}, 1, gfx.Point{})
	b, hb := newNode(2, gfx.Size{W: 10, H: 20}, gfx.Size{W: 80, H: 20}, 1, gfx.Point{})

	p.Arrange([]layout.ChildNode{a, b}, layout.ResolvedLayer{Bounds: gfx.RectFromXYWH(0, 0, 150, 100)})

	if got, ok := ha.Bounds(); !ok || got.Width() < 120 {
		t.Fatalf("unexpected first width: %#v", got)
	}
	if got, ok := hb.Bounds(); !ok || got.Width() < 80 {
		t.Fatalf("unexpected second width: %#v", got)
	}
}

func TestVerticalSplitNonStructural_arranges_without_measure(t *testing.T) {
	p := New(Config{Axis: Vertical, DividerSize: 2})
	a, ha := newNode(1, gfx.Size{W: 20, H: 10}, gfx.Size{}, 0, gfx.Point{Y: 40})
	b, hb := newNode(2, gfx.Size{W: 20, H: 10}, gfx.Size{}, 1, gfx.Point{})

	p.Arrange([]layout.ChildNode{a, b}, layout.ResolvedLayer{Bounds: gfx.RectFromXYWH(0, 0, 100, 200)})

	if got, ok := ha.Bounds(); !ok || got.Height() != 40 {
		t.Fatalf("unexpected first bounds: %#v", got)
	}
	if got, ok := hb.Bounds(); !ok || got.Min.Y != 42 {
		t.Fatalf("unexpected second bounds: %#v", got)
	}
}

func TestZeroChildren(t *testing.T) {
	p := New(Config{Axis: Horizontal})
	if got := p.Measure(nil, gfx.Size{W: 100, H: 100}); got != (gfx.Size{}) {
		t.Fatalf("unexpected size: %#v", got)
	}
	p.Arrange(nil, layout.ResolvedLayer{Bounds: gfx.RectFromXYWH(0, 0, 100, 100)})
}

func TestSinglePane_gets_full_extent(t *testing.T) {
	p := New(Config{Axis: Horizontal})
	child, handle := newNode(1, gfx.Size{W: 20, H: 10}, gfx.Size{}, 1, gfx.Point{})
	p.Arrange([]layout.ChildNode{child}, layout.ResolvedLayer{Bounds: gfx.RectFromXYWH(0, 0, 100, 20)})
	got, ok := handle.Bounds()
	if !ok || got.Width() != 100 {
		t.Fatalf("unexpected bounds: %#v", got)
	}
}

func TestAllFixedOverflows_without_panic(t *testing.T) {
	p := New(Config{Axis: Horizontal, DividerSize: 10})
	a, _ := newNode(1, gfx.Size{W: 60, H: 10}, gfx.Size{}, 0, gfx.Point{X: 80})
	b, _ := newNode(2, gfx.Size{W: 60, H: 10}, gfx.Size{}, 0, gfx.Point{X: 80})
	p.Arrange([]layout.ChildNode{a, b}, layout.ResolvedLayer{Bounds: gfx.RectFromXYWH(0, 0, 100, 20)})
}

func TestMeasureAndArrangeStable(t *testing.T) {
	p := New(Config{Axis: Horizontal, DividerSize: 2})
	a1, h1 := newNode(1, gfx.Size{W: 10, H: 20}, gfx.Size{}, 1, gfx.Point{})
	b1, h2 := newNode(2, gfx.Size{W: 10, H: 20}, gfx.Size{}, 2, gfx.Point{})
	children1 := []layout.ChildNode{a1, b1}

	a2, h3 := newNode(1, gfx.Size{W: 10, H: 20}, gfx.Size{}, 1, gfx.Point{})
	b2, h4 := newNode(2, gfx.Size{W: 10, H: 20}, gfx.Size{}, 2, gfx.Point{})
	children2 := []layout.ChildNode{a2, b2}

	bounds := layout.ResolvedLayer{Bounds: gfx.RectFromXYWH(0, 0, 300, 100)}
	p.Arrange(children1, bounds)
	p.Arrange(children2, bounds)

	got1, ok1 := h1.Bounds()
	got2, ok2 := h2.Bounds()
	got3, ok3 := h3.Bounds()
	got4, ok4 := h4.Bounds()
	if !ok1 || !ok2 || !ok3 || !ok4 {
		t.Fatal("expected all arranged bounds to be present")
	}
	if got1 != got3 || got2 != got4 {
		t.Fatalf("unexpected instability: %#v %#v %#v %#v", got1, got2, got3, got4)
	}
}

func BenchmarkPolicyArrange(b *testing.B) {
	p := New(Config{Axis: Horizontal, DividerSize: 2})
	children := make([]layout.ChildNode, 0, 16)
	handles := make([]*layout.ChildArrangeHandle, 0, 16)
	for i := 0; i < 16; i++ {
		node, handle := newNode(facet.FacetID(i+1), gfx.Size{W: 10, H: 10}, gfx.Size{}, 1, gfx.Point{})
		children = append(children, node)
		handles = append(handles, handle)
	}
	layer := layout.ResolvedLayer{Bounds: gfx.RectFromXYWH(0, 0, 800, 100)}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		p.Arrange(children, layer)
		for _, handle := range handles {
			*handle = layout.ChildArrangeHandle{}
		}
	}
}

func almostEqual(a, b float32) bool {
	const eps = 0.0001
	if a > b {
		return a-b < eps
	}
	return b-a < eps
}
