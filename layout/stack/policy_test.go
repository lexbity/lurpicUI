package stack

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
)

func newChild(id facet.FacetID, intrinsic, min gfx.Size, flex float32, align layout.Alignment) (layout.ChildNode, *layout.ChildArrangeHandle) {
	handle := &layout.ChildArrangeHandle{}
	node := layout.ChildNode{
		FacetID: id,
		Attachment: layout.ChildAttachment{
			Placement: layout.PlacementHints{Flex: flex, Align: align},
		},
		IntrinsicSize: intrinsic,
		MinSize:       min,
	}
	node.AttachArrangeHandle(handle)
	return node, handle
}

func measure(p *Policy, children []layout.ChildNode, size gfx.Size) gfx.Size {
	return p.Measure(children, size)
}

func arrange(p *Policy, children []layout.ChildNode, bounds gfx.Rect) {
	p.Arrange(children, layout.ResolvedLayer{Bounds: bounds})
}

func TestVerticalStackStructural_fixed_sizes(t *testing.T) {
	p := New(Config{Axis: Vertical})
	children := []layout.ChildNode{}
	a, _ := newChild(1, gfx.Size{W: 40, H: 20}, gfx.Size{}, 0, layout.AlignStart)
	b, _ := newChild(2, gfx.Size{W: 60, H: 35}, gfx.Size{}, 0, layout.AlignStart)
	children = append(children, a, b)

	got := measure(p, children, gfx.Size{W: 200, H: 200})
	if got != (gfx.Size{W: 60, H: 55}) {
		t.Fatalf("unexpected size: %#v", got)
	}
}

func TestVerticalStackStructural_flex_distribution(t *testing.T) {
	p := New(Config{Axis: Vertical, CrossAlign: layout.AlignStart})
	a, ha := newChild(1, gfx.Size{W: 40, H: 20}, gfx.Size{}, 0, layout.AlignStart)
	b, hb := newChild(2, gfx.Size{W: 40, H: 20}, gfx.Size{}, 1, layout.AlignStart)
	children := []layout.ChildNode{a, b}

	arrange(p, children, gfx.RectFromXYWH(0, 0, 100, 200))

	if got, ok := ha.Bounds(); !ok || got != (gfx.RectFromXYWH(0, 0, 40, 20)) {
		t.Fatalf("unexpected first bounds: %#v", got)
	}
	if got, ok := hb.Bounds(); !ok || got != (gfx.RectFromXYWH(0, 20, 40, 180)) {
		t.Fatalf("unexpected second bounds: %#v", got)
	}
}

func TestVerticalStackStructural_twoFlexWeights(t *testing.T) {
	p := New(Config{Axis: Vertical, CrossAlign: layout.AlignStart})
	a, ha := newChild(1, gfx.Size{W: 40, H: 20}, gfx.Size{}, 1, layout.AlignStart)
	b, hb := newChild(2, gfx.Size{W: 40, H: 20}, gfx.Size{}, 2, layout.AlignStart)
	children := []layout.ChildNode{a, b}

	arrange(p, children, gfx.RectFromXYWH(0, 0, 100, 200))

	if got, ok := ha.Bounds(); !ok || !almostEqual(got.Height(), 73.333336) {
		t.Fatalf("unexpected first height: %v", got)
	}
	if got, ok := hb.Bounds(); !ok || !almostEqual(got.Height(), 126.666664) {
		t.Fatalf("unexpected second height: %v", got)
	}
}

func TestVerticalStackCrossAlignment(t *testing.T) {
	cases := []struct {
		name  string
		align layout.Alignment
		wantX float32
		wantW float32
	}{
		{name: "start", align: layout.AlignStart, wantX: 0, wantW: 20},
		{name: "center", align: layout.AlignCenter, wantX: 40, wantW: 20},
		{name: "end", align: layout.AlignEnd, wantX: 80, wantW: 20},
		{name: "stretch", align: layout.AlignStretch, wantX: 0, wantW: 100},
	}
	for _, tc := range cases {
		p := New(Config{Axis: Vertical, CrossAlign: tc.align})
		child, handle := newChild(1, gfx.Size{W: 20, H: 10}, gfx.Size{}, 0, layout.AlignStart)
		arrange(p, []layout.ChildNode{child}, gfx.RectFromXYWH(0, 0, 100, 50))
		got, ok := handle.Bounds()
		if !ok {
			t.Fatalf("%s: missing bounds", tc.name)
		}
		if got.Min.X != tc.wantX || got.Width() != tc.wantW {
			t.Fatalf("%s: unexpected bounds %#v", tc.name, got)
		}
	}
}

func TestHorizontalStackStructural_fixed_sizes(t *testing.T) {
	p := New(Config{Axis: Horizontal})
	a, _ := newChild(1, gfx.Size{W: 20, H: 40}, gfx.Size{}, 0, layout.AlignStart)
	b, _ := newChild(2, gfx.Size{W: 35, H: 60}, gfx.Size{}, 0, layout.AlignStart)
	got := measure(p, []layout.ChildNode{a, b}, gfx.Size{W: 200, H: 200})
	if got != (gfx.Size{W: 55, H: 60}) {
		t.Fatalf("unexpected size: %#v", got)
	}
}

func TestHorizontalStackNonStructural_arranges_without_measure(t *testing.T) {
	p := New(Config{Axis: Horizontal, CrossAlign: layout.AlignStart})
	a, ha := newChild(1, gfx.Size{W: 20, H: 10}, gfx.Size{}, 0, layout.AlignStart)
	b, hb := newChild(2, gfx.Size{W: 30, H: 10}, gfx.Size{}, 1, layout.AlignStart)
	children := []layout.ChildNode{a, b}

	arrange(p, children, gfx.RectFromXYWH(0, 0, 200, 50))

	if got, ok := ha.Bounds(); !ok || got != (gfx.RectFromXYWH(0, 0, 20, 10)) {
		t.Fatalf("unexpected first bounds: %#v", got)
	}
	if got, ok := hb.Bounds(); !ok || got != (gfx.RectFromXYWH(20, 0, 180, 10)) {
		t.Fatalf("unexpected second bounds: %#v", got)
	}
}

func TestMainAlignmentSpaceBetween(t *testing.T) {
	p := New(Config{Axis: Horizontal, MainAlign: MainSpaceBetween})
	a, ha := newChild(1, gfx.Size{W: 20, H: 10}, gfx.Size{}, 0, layout.AlignStart)
	b, hb := newChild(2, gfx.Size{W: 20, H: 10}, gfx.Size{}, 0, layout.AlignStart)
	c, hc := newChild(3, gfx.Size{W: 20, H: 10}, gfx.Size{}, 0, layout.AlignStart)
	arrange(p, []layout.ChildNode{a, b, c}, gfx.RectFromXYWH(0, 0, 120, 20))

	if got, ok := ha.Bounds(); !ok || got.Min.X != 0 {
		t.Fatalf("unexpected first x: %v", got)
	}
	if got, ok := hb.Bounds(); !ok || got.Min.X != 50 {
		t.Fatalf("unexpected second x: %v", got)
	}
	if got, ok := hc.Bounds(); !ok || got.Min.X != 100 {
		t.Fatalf("unexpected third x: %v", got)
	}
}

func TestMainAlignmentSpaceAround(t *testing.T) {
	p := New(Config{Axis: Horizontal, MainAlign: MainSpaceAround})
	a, ha := newChild(1, gfx.Size{W: 20, H: 10}, gfx.Size{}, 0, layout.AlignStart)
	b, hb := newChild(2, gfx.Size{W: 20, H: 10}, gfx.Size{}, 0, layout.AlignStart)
	c, hc := newChild(3, gfx.Size{W: 20, H: 10}, gfx.Size{}, 0, layout.AlignStart)
	arrange(p, []layout.ChildNode{a, b, c}, gfx.RectFromXYWH(0, 0, 120, 20))

	if got, ok := ha.Bounds(); !ok || got.Min.X != 10 {
		t.Fatalf("unexpected first x: %v", got)
	}
	if got, ok := hb.Bounds(); !ok || got.Min.X != 50 {
		t.Fatalf("unexpected second x: %v", got)
	}
	if got, ok := hc.Bounds(); !ok || got.Min.X != 90 {
		t.Fatalf("unexpected third x: %v", got)
	}
}

func TestMainAlignmentCenter(t *testing.T) {
	p := New(Config{Axis: Horizontal, MainAlign: MainCenter})
	a, ha := newChild(1, gfx.Size{W: 20, H: 10}, gfx.Size{}, 0, layout.AlignStart)
	b, hb := newChild(2, gfx.Size{W: 20, H: 10}, gfx.Size{}, 0, layout.AlignStart)
	arrange(p, []layout.ChildNode{a, b}, gfx.RectFromXYWH(0, 0, 100, 20))

	if got, ok := ha.Bounds(); !ok || got.Min.X != 30 {
		t.Fatalf("unexpected first x: %v", got)
	}
	if got, ok := hb.Bounds(); !ok || got.Min.X != 50 {
		t.Fatalf("unexpected second x: %v", got)
	}
}

func TestZeroChildren(t *testing.T) {
	p := New(Config{Axis: Vertical})
	if got := p.Measure(nil, gfx.Size{W: 100, H: 100}); got != (gfx.Size{}) {
		t.Fatalf("unexpected size: %#v", got)
	}
	p.Arrange(nil, layout.ResolvedLayer{Bounds: gfx.RectFromXYWH(0, 0, 100, 100)})
}

func TestSingleFlexChildGetsFullExtent(t *testing.T) {
	p := New(Config{Axis: Horizontal, CrossAlign: layout.AlignStart})
	child, handle := newChild(1, gfx.Size{W: 20, H: 10}, gfx.Size{}, 1, layout.AlignStart)
	arrange(p, []layout.ChildNode{child}, gfx.RectFromXYWH(0, 0, 100, 20))
	if got, ok := handle.Bounds(); !ok || got != (gfx.RectFromXYWH(0, 0, 100, 10)) {
		t.Fatalf("unexpected bounds: %#v", got)
	}
}

func TestOverflowStillArrangesChildren(t *testing.T) {
	p := New(Config{Axis: Horizontal, Spacing: 10})
	a, ha := newChild(1, gfx.Size{W: 60, H: 10}, gfx.Size{}, 0, layout.AlignStart)
	b, hb := newChild(2, gfx.Size{W: 60, H: 10}, gfx.Size{}, 0, layout.AlignStart)
	arrange(p, []layout.ChildNode{a, b}, gfx.RectFromXYWH(0, 0, 100, 20))
	if got, ok := ha.Bounds(); !ok || got.Min.X != 0 {
		t.Fatalf("unexpected first x: %v", got)
	}
	if got, ok := hb.Bounds(); !ok || got.Min.X != 70 {
		t.Fatalf("unexpected second x: %v", got)
	}
}

func TestIdempotentArrange(t *testing.T) {
	p := New(Config{Axis: Horizontal, MainAlign: MainStart})
	makeChildren := func() ([]layout.ChildNode, []*layout.ChildArrangeHandle) {
		a, ha := newChild(1, gfx.Size{W: 20, H: 10}, gfx.Size{}, 1, layout.AlignStart)
		b, hb := newChild(2, gfx.Size{W: 30, H: 10}, gfx.Size{}, 2, layout.AlignStart)
		return []layout.ChildNode{a, b}, []*layout.ChildArrangeHandle{ha, hb}
	}
	children1, handles1 := makeChildren()
	children2, handles2 := makeChildren()
	bounds := gfx.RectFromXYWH(0, 0, 100, 20)
	arrange(p, children1, bounds)
	arrange(p, children2, bounds)

	for i := range handles1 {
		got, ok1 := handles1[i].Bounds()
		want, ok2 := handles2[i].Bounds()
		if !ok1 || !ok2 || got != want {
			t.Fatalf("arrange mismatch: got %#v want %#v", got, want)
		}
	}
}

func BenchmarkPolicyArrange(b *testing.B) {
	p := New(Config{Axis: Horizontal, MainAlign: MainStart, CrossAlign: layout.AlignStretch})
	children := make([]layout.ChildNode, 0, 16)
	handles := make([]*layout.ChildArrangeHandle, 0, 16)
	for i := 0; i < 16; i++ {
		node, handle := newChild(facet.FacetID(i+1), gfx.Size{W: 10, H: 10}, gfx.Size{}, 1, layout.AlignStart)
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
