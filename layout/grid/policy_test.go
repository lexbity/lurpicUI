package grid

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
)

func newNode(id facet.FacetID, intrinsic, min gfx.Size, colStart, colSpan, rowStart, rowSpan, z int, align layout.Alignment) (layout.ChildNode, *layout.ChildArrangeHandle) {
	handle := &layout.ChildArrangeHandle{}
	node := layout.ChildNode{
		FacetID: id,
		Attachment: layout.ChildAttachment{
			ZPriority: z,
			Placement: layout.PlacementHints{
				ColStart: colStart,
				ColSpan:  colSpan,
				RowStart: rowStart,
				RowSpan:  rowSpan,
				Align:    align,
			},
		},
		IntrinsicSize: intrinsic,
		MinSize:       min,
	}
	node.AttachArrangeHandle(handle)
	return node, handle
}

func TestStructuralMeasure_intrinsicTracks(t *testing.T) {
	p := New(Config{
		Columns: []TrackDef{{Sizing: TrackIntrinsic}, {Sizing: TrackIntrinsic}},
		Rows:    []TrackDef{{Sizing: TrackIntrinsic}},
	})
	a, _ := newNode(1, gfx.Size{W: 40, H: 20}, gfx.Size{}, 1, 1, 1, 1, 0, layout.AlignStretch)
	b, _ := newNode(2, gfx.Size{W: 60, H: 30}, gfx.Size{}, 2, 1, 1, 1, 0, layout.AlignStretch)

	got := p.Measure([]layout.ChildNode{a, b}, gfx.Size{W: 200, H: 100})
	if got != (gfx.Size{W: 100, H: 30}) {
		t.Fatalf("unexpected size: %#v", got)
	}
}

func TestStructuralMeasure_fixedAndFlex(t *testing.T) {
	p := New(Config{
		Columns: []TrackDef{
			{Sizing: TrackFixed, Value: 100},
			{Sizing: TrackFlex, Value: 1, Min: 0},
		},
		Rows: []TrackDef{{Sizing: TrackIntrinsic}},
	})
	a, _ := newNode(1, gfx.Size{W: 10, H: 20}, gfx.Size{}, 1, 1, 1, 1, 0, layout.AlignStretch)
	b, _ := newNode(2, gfx.Size{W: 10, H: 20}, gfx.Size{}, 2, 1, 1, 1, 0, layout.AlignStretch)

	got := p.Measure([]layout.ChildNode{a, b}, gfx.Size{W: 300, H: 100})
	if got.W != 300 {
		t.Fatalf("unexpected width: %#v", got)
	}
}

func TestSpanContributesAcrossTracks(t *testing.T) {
	p := New(Config{
		Columns: []TrackDef{{Sizing: TrackIntrinsic}, {Sizing: TrackIntrinsic}, {Sizing: TrackIntrinsic}},
		Rows:    []TrackDef{{Sizing: TrackIntrinsic}},
	})
	a, _ := newNode(1, gfx.Size{W: 120, H: 20}, gfx.Size{}, 1, 2, 1, 1, 0, layout.AlignStretch)
	b, _ := newNode(2, gfx.Size{W: 30, H: 20}, gfx.Size{}, 3, 1, 1, 1, 0, layout.AlignStretch)

	got := p.Measure([]layout.ChildNode{a, b}, gfx.Size{W: 200, H: 100})
	if got.W != 150 {
		t.Fatalf("unexpected width: %#v", got)
	}
}

func TestAutoPlacementRowFirst_deterministicByZPriority(t *testing.T) {
	p := New(Config{
		Columns:       []TrackDef{{Sizing: TrackFlex, Value: 1}, {Sizing: TrackFlex, Value: 1}},
		Rows:          []TrackDef{{Sizing: TrackFlex, Value: 1}, {Sizing: TrackFlex, Value: 1}},
		AutoPlacement: AutoRowFirst,
	})
	a, ha := newNode(1, gfx.Size{W: 10, H: 10}, gfx.Size{}, 0, 0, 0, 0, 2, layout.AlignStretch)
	b, hb := newNode(2, gfx.Size{W: 10, H: 10}, gfx.Size{}, 0, 0, 0, 0, 0, layout.AlignStretch)
	c, hc := newNode(3, gfx.Size{W: 10, H: 10}, gfx.Size{}, 0, 0, 0, 0, 1, layout.AlignStretch)

	p.Arrange([]layout.ChildNode{a, b, c}, layout.ResolvedLayer{Bounds: gfx.RectFromXYWH(0, 0, 200, 200)})

	if got, ok := hb.Bounds(); !ok || got.Min != (gfx.Point{X: 0, Y: 0}) {
		t.Fatalf("unexpected first auto child: %#v", got)
	}
	if got, ok := hc.Bounds(); !ok || got.Min != (gfx.Point{X: 100, Y: 0}) {
		t.Fatalf("unexpected second auto child: %#v", got)
	}
	if got, ok := ha.Bounds(); !ok || got.Min != (gfx.Point{X: 0, Y: 100}) {
		t.Fatalf("unexpected third auto child: %#v", got)
	}
}

func TestCellAlignment(t *testing.T) {
	cases := []struct {
		name  string
		align layout.Alignment
		want  gfx.Rect
	}{
		{name: "stretch", align: layout.AlignStretch, want: gfx.RectFromXYWH(0, 0, 100, 100)},
		{name: "start", align: layout.AlignStart, want: gfx.RectFromXYWH(0, 0, 20, 10)},
		{name: "center", align: layout.AlignCenter, want: gfx.RectFromXYWH(40, 45, 20, 10)},
		{name: "end", align: layout.AlignEnd, want: gfx.RectFromXYWH(80, 90, 20, 10)},
	}
	for _, tc := range cases {
		p := New(Config{
			Columns: []TrackDef{{Sizing: TrackFlex, Value: 1}},
			Rows:    []TrackDef{{Sizing: TrackFlex, Value: 1}},
		})
		child, handle := newNode(1, gfx.Size{W: 20, H: 10}, gfx.Size{}, 1, 1, 1, 1, 0, tc.align)
		p.Arrange([]layout.ChildNode{child}, layout.ResolvedLayer{Bounds: gfx.RectFromXYWH(0, 0, 100, 100)})
		got, ok := handle.Bounds()
		if !ok || got != tc.want {
			t.Fatalf("%s: unexpected bounds %#v", tc.name, got)
		}
	}
}

func TestNonStructuralArrange_usesLayerBounds(t *testing.T) {
	p := New(Config{
		Columns: []TrackDef{{Sizing: TrackFlex, Value: 1}, {Sizing: TrackFlex, Value: 1}},
		Rows:    []TrackDef{{Sizing: TrackFlex, Value: 1}},
	})
	a, ha := newNode(1, gfx.Size{W: 10, H: 10}, gfx.Size{}, 1, 1, 1, 1, 0, layout.AlignStretch)
	b, hb := newNode(2, gfx.Size{W: 10, H: 10}, gfx.Size{}, 2, 1, 1, 1, 0, layout.AlignStretch)

	p.Arrange([]layout.ChildNode{a, b}, layout.ResolvedLayer{Bounds: gfx.RectFromXYWH(0, 0, 200, 100)})

	if got, ok := ha.Bounds(); !ok || got.Width() != 100 {
		t.Fatalf("unexpected first bounds: %#v", got)
	}
	if got, ok := hb.Bounds(); !ok || got.Min.X != 100 {
		t.Fatalf("unexpected second bounds: %#v", got)
	}
}

func TestEdgeCases(t *testing.T) {
	p := New(Config{
		Columns: []TrackDef{{Sizing: TrackFixed, Value: 50}, {Sizing: TrackFixed, Value: 50}},
		Rows:    []TrackDef{{Sizing: TrackFixed, Value: 50}},
	})
	child, handle := newNode(1, gfx.Size{W: 20, H: 10}, gfx.Size{}, 9, 1, 1, 1, 0, layout.AlignStretch)
	p.Arrange([]layout.ChildNode{child}, layout.ResolvedLayer{Bounds: gfx.RectFromXYWH(0, 0, 100, 50)})
	got, ok := handle.Bounds()
	if !ok || got.Min.X != 50 {
		t.Fatalf("unexpected clamped placement: %#v", got)
	}
}

func TestZeroChildren(t *testing.T) {
	p := New(Config{})
	if got := p.Measure(nil, gfx.Size{W: 100, H: 100}); got != (gfx.Size{}) {
		t.Fatalf("unexpected size: %#v", got)
	}
	p.Arrange(nil, layout.ResolvedLayer{Bounds: gfx.RectFromXYWH(0, 0, 100, 100)})
}

func TestSingleCellGrid_fullBounds(t *testing.T) {
	p := New(Config{
		Columns: []TrackDef{{Sizing: TrackFlex, Value: 1}},
		Rows:    []TrackDef{{Sizing: TrackFlex, Value: 1}},
	})
	child, handle := newNode(1, gfx.Size{W: 10, H: 10}, gfx.Size{}, 1, 1, 1, 1, 0, layout.AlignStretch)
	p.Arrange([]layout.ChildNode{child}, layout.ResolvedLayer{Bounds: gfx.RectFromXYWH(0, 0, 100, 100)})
	got, ok := handle.Bounds()
	if !ok || got != (gfx.RectFromXYWH(0, 0, 100, 100)) {
		t.Fatalf("unexpected bounds: %#v", got)
	}
}

func BenchmarkPolicyArrange(b *testing.B) {
	p := New(Config{
		Columns: []TrackDef{{Sizing: TrackFlex, Value: 1}, {Sizing: TrackFlex, Value: 1}, {Sizing: TrackFlex, Value: 1}, {Sizing: TrackFlex, Value: 1}},
		Rows:    []TrackDef{{Sizing: TrackFlex, Value: 1}, {Sizing: TrackFlex, Value: 1}, {Sizing: TrackFlex, Value: 1}, {Sizing: TrackFlex, Value: 1}},
	})
	children := make([]layout.ChildNode, 0, 16)
	handles := make([]*layout.ChildArrangeHandle, 0, 16)
	for i := 0; i < 16; i++ {
		col := (i % 4) + 1
		row := (i / 4) + 1
		node, handle := newNode(facet.FacetID(i+1), gfx.Size{W: 10, H: 10}, gfx.Size{}, col, 1, row, 1, i, layout.AlignStretch)
		children = append(children, node)
		handles = append(handles, handle)
	}
	layer := layout.ResolvedLayer{Bounds: gfx.RectFromXYWH(0, 0, 800, 800)}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		p.Arrange(children, layer)
		for _, handle := range handles {
			*handle = layout.ChildArrangeHandle{}
		}
	}
}
