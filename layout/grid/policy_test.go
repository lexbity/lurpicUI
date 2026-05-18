package grid

import (
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

type testGridChild struct {
	child   Child
	layout  *facet.LayoutRole
	arrange gfx.Rect
}

func newTestGridChild(id facet.FacetID, size gfx.Size, placement facet.Placement, z int32) testGridChild {
	role := &facet.LayoutRole{}
	role.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: size}
	}
	role.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		role.ArrangedBounds = bounds
	}
	role.Child.SupportedPlacement = facet.SupportsGrid
	return testGridChild{
		child: Child{
			FacetID: id,
			Attachment: facet.Attachment{
				LayerID:   facet.LayerID(1),
				Placement: placement,
				ZPriority: z,
			},
			Layout:   role,
			Contract: role.Child,
		},
		layout: role,
	}
}

func TestPolicy_equalFlexTracks(t *testing.T) {
	p := New(Config{
		Columns: []TrackDef{{Sizing: TrackFlex, Value: 1}, {Sizing: TrackFlex, Value: 1}},
		Rows:    []TrackDef{{Sizing: TrackFlex, Value: 1}},
	})
	first := newTestGridChild(1, gfx.Size{W: 20, H: 10}, facet.Placement{Mode: facet.PlacementGrid}, 1)
	second := newTestGridChild(2, gfx.Size{W: 20, H: 10}, facet.Placement{Mode: facet.PlacementGrid}, 0)

	size, err := p.Measure([]Child{first.child, second.child}, gfx.Size{W: 200, H: 100})
	if err != nil {
		t.Fatalf("Measure: %v", err)
	}
	if size != (gfx.Size{W: 200, H: 100}) {
		t.Fatalf("measure = %#v, want 200x100", size)
	}
	arranged, err := p.Arrange([]Child{first.child, second.child}, gfx.RectFromXYWH(0, 0, 200, 100))
	if err != nil {
		t.Fatalf("Arrange: %v", err)
	}
	if len(arranged) != 2 {
		t.Fatalf("arranged count = %d, want 2", len(arranged))
	}
	if arranged[0].Bounds != (gfx.RectFromXYWH(0, 0, 100, 100)) {
		t.Fatalf("first bounds = %#v", arranged[0].Bounds)
	}
	if arranged[1].Bounds != (gfx.RectFromXYWH(100, 0, 100, 100)) {
		t.Fatalf("second bounds = %#v", arranged[1].Bounds)
	}
}

func TestPolicy_intrinsicTracks(t *testing.T) {
	p := New(Config{
		Columns: []TrackDef{{Sizing: TrackIntrinsic}, {Sizing: TrackIntrinsic}},
		Rows:    []TrackDef{{Sizing: TrackIntrinsic}},
	})
	first := newTestGridChild(1, gfx.Size{W: 40, H: 20}, facet.Placement{Mode: facet.PlacementGrid}, 0)
	first.child.Attachment.Placement.Grid = facet.GridPlacement{ColStart: 0, RowStart: 0, ColSpan: 1, RowSpan: 1}
	second := newTestGridChild(2, gfx.Size{W: 60, H: 30}, facet.Placement{Mode: facet.PlacementGrid}, 0)
	second.child.Attachment.Placement.Grid = facet.GridPlacement{ColStart: 1, RowStart: 0, ColSpan: 1, RowSpan: 1}

	size, err := p.Measure([]Child{first.child, second.child}, gfx.Size{W: 200, H: 100})
	if err != nil {
		t.Fatalf("Measure: %v", err)
	}
	if size != (gfx.Size{W: 100, H: 30}) {
		t.Fatalf("measure = %#v, want 100x30", size)
	}
	arranged, err := p.Arrange([]Child{first.child, second.child}, gfx.RectFromXYWH(0, 0, 100, 30))
	if err != nil {
		t.Fatalf("Arrange: %v", err)
	}
	if arranged[0].Bounds != (gfx.RectFromXYWH(0, 0, 40, 30)) {
		t.Fatalf("first bounds = %#v", arranged[0].Bounds)
	}
	if arranged[1].Bounds != (gfx.RectFromXYWH(40, 0, 60, 30)) {
		t.Fatalf("second bounds = %#v", arranged[1].Bounds)
	}
}

func TestPolicy_mixedTracks(t *testing.T) {
	p := New(Config{
		Columns: []TrackDef{{Sizing: TrackFixed, Value: 50}, {Sizing: TrackFlex, Value: 1}},
		Rows:    []TrackDef{{Sizing: TrackFlex, Value: 1}},
	})
	child := newTestGridChild(1, gfx.Size{W: 20, H: 10}, facet.Placement{Mode: facet.PlacementGrid}, 0)
	child.child.Attachment.Placement.Grid = facet.GridPlacement{ColStart: 1, RowStart: 0, ColSpan: 1, RowSpan: 1}
	other := newTestGridChild(2, gfx.Size{W: 20, H: 10}, facet.Placement{Mode: facet.PlacementGrid}, 0)
	other.child.Attachment.Placement.Grid = facet.GridPlacement{ColStart: 0, RowStart: 0, ColSpan: 1, RowSpan: 1}

	size, err := p.Measure([]Child{child.child, other.child}, gfx.Size{W: 250, H: 100})
	if err != nil {
		t.Fatalf("Measure: %v", err)
	}
	if size != (gfx.Size{W: 250, H: 100}) {
		t.Fatalf("measure = %#v, want 250x100", size)
	}
	arranged, err := p.Arrange([]Child{other.child, child.child}, gfx.RectFromXYWH(0, 0, 250, 100))
	if err != nil {
		t.Fatalf("Arrange: %v", err)
	}
	if arranged[0].Bounds != (gfx.RectFromXYWH(0, 0, 50, 100)) {
		t.Fatalf("first bounds = %#v", arranged[0].Bounds)
	}
	if arranged[1].Bounds != (gfx.RectFromXYWH(50, 0, 200, 100)) {
		t.Fatalf("second bounds = %#v", arranged[1].Bounds)
	}
}

func TestPolicy_autoPlacementIsDeterministic(t *testing.T) {
	p := New(Config{
		Columns: []TrackDef{{Sizing: TrackFlex, Value: 1}, {Sizing: TrackFlex, Value: 1}},
		Rows:    []TrackDef{{Sizing: TrackFlex, Value: 1}, {Sizing: TrackFlex, Value: 1}},
	})
	a := newTestGridChild(1, gfx.Size{W: 10, H: 10}, facet.Placement{Mode: facet.PlacementGrid}, 2)
	b := newTestGridChild(2, gfx.Size{W: 10, H: 10}, facet.Placement{Mode: facet.PlacementGrid}, 0)
	c := newTestGridChild(3, gfx.Size{W: 10, H: 10}, facet.Placement{Mode: facet.PlacementGrid}, 1)

	arranged, err := p.Arrange([]Child{a.child, b.child, c.child}, gfx.RectFromXYWH(0, 0, 200, 200))
	if err != nil {
		t.Fatalf("Arrange: %v", err)
	}
	byID := make(map[facet.FacetID]ArrangedChild, len(arranged))
	for i := range arranged {
		byID[arranged[i].FacetID] = arranged[i]
	}
	if got := byID[1]; got.Bounds.Min != (gfx.Point{X: 0, Y: 0}) {
		t.Fatalf("facet 1 arranged child = %#v", got)
	}
	if got := byID[3]; got.Bounds.Min != (gfx.Point{X: 100, Y: 0}) {
		t.Fatalf("facet 3 arranged child = %#v", got)
	}
	if got := byID[2]; got.Bounds.Min != (gfx.Point{X: 0, Y: 100}) {
		t.Fatalf("facet 2 arranged child = %#v", got)
	}
}

func TestPolicy_rejectsInvalidSpan(t *testing.T) {
	p := New(Config{
		Columns: []TrackDef{{Sizing: TrackFlex, Value: 1}},
		Rows:    []TrackDef{{Sizing: TrackFlex, Value: 1}},
	})
	child := newTestGridChild(1, gfx.Size{W: 10, H: 10}, facet.Placement{Mode: facet.PlacementGrid}, 0)
	child.child.Attachment.Placement.Grid = facet.GridPlacement{ColStart: 0, RowStart: 0, ColSpan: 0, RowSpan: 1}
	_, err := p.Arrange([]Child{child.child}, gfx.RectFromXYWH(0, 0, 100, 100))
	if err == nil || !strings.Contains(err.Error(), "span") {
		t.Fatalf("Arrange error = %v, want span validation", err)
	}
}

func TestPolicy_rejectsOutOfBoundsLine(t *testing.T) {
	p := New(Config{
		Columns: []TrackDef{{Sizing: TrackFlex, Value: 1}},
		Rows:    []TrackDef{{Sizing: TrackFlex, Value: 1}},
	})
	child := newTestGridChild(1, gfx.Size{W: 10, H: 10}, facet.Placement{Mode: facet.PlacementGrid}, 0)
	child.child.Attachment.Placement.Grid = facet.GridPlacement{ColStart: 1, RowStart: 0, ColSpan: 1, RowSpan: 1}
	_, err := p.Arrange([]Child{child.child}, gfx.RectFromXYWH(0, 0, 100, 100))
	if err == nil || !strings.Contains(err.Error(), "outside track range") {
		t.Fatalf("Arrange error = %v, want out-of-bounds validation", err)
	}
}
