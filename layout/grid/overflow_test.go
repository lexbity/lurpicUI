package grid

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

// newOverflowTestChild builds a grid child with a fixed measured size.
func newOverflowTestChild(id facet.FacetID, size gfx.Size) Child {
	role := &facet.LayoutRole{}
	role.MeasuredSize = size
	return Child{
		FacetID: id,
		Layout:  role,
		Attachment: facet.Attachment{
			Placement: facet.Placement{Mode: facet.PlacementGrid, Grid: facet.GridPlacement{ColStart: 0, RowStart: 0, ColSpan: 1, RowSpan: 1}},
		},
		Contract: facet.GroupChildContract{SupportedPlacement: facet.SupportsGrid},
	}
}

// TestOverflowTracks_flexTrackCompressedBelowContent pins RX-1 FR-5 per-track
// overflow reporting: a flex track compressed to a thin band while its child
// needs more width is reported (need > size). Intrinsic tracks never report.
func TestOverflowTracks_flexTrackCompressedBelowContent(t *testing.T) {
	policy := New(Config{
		Columns: []TrackDef{{Sizing: TrackFlex, Value: 1, Min: 0}},
		Rows:    []TrackDef{{Sizing: TrackIntrinsic}},
	})
	child := newOverflowTestChild(1, gfx.Size{W: 120, H: 40})
	layer := gfx.RectFromXYWH(0, 0, 40, 40)

	out := policy.OverflowTracks([]Child{child}, layer)
	if len(out) == 0 {
		t.Fatal("expected a column overflow report for a compressed flex track")
	}
	col := out[0]
	if !col.Horizontal {
		t.Fatalf("reported overflow not on the column axis: %+v", col)
	}
	if col.Index != 0 {
		t.Fatalf("overflow track index = %d, want 0", col.Index)
	}
	if col.Need != 120 {
		t.Fatalf("overflow need = %v, want 120", col.Need)
	}
	if col.Size > col.Need {
		t.Fatalf("overflow size %v should be below need %v", col.Size, col.Need)
	}
}

// TestOverflowTracks_intrinsicTrackNoReport pins that an intrinsic track is
// sized to its content and never reports overflow.
func TestOverflowTracks_intrinsicTrackNoReport(t *testing.T) {
	policy := New(Config{
		Columns: []TrackDef{{Sizing: TrackIntrinsic}},
		Rows:    []TrackDef{{Sizing: TrackIntrinsic}},
	})
	child := newOverflowTestChild(2, gfx.Size{W: 120, H: 40})
	layer := gfx.RectFromXYWH(0, 0, 200, 200)

	out := policy.OverflowTracks([]Child{child}, layer)
	if len(out) != 0 {
		t.Fatalf("intrinsic tracks reported overflow: %+v", out)
	}
}

// TestOverflowTracks_fixedTrackOverflow pins a fixed track narrower than its
// child is reported as overflow.
func TestOverflowTracks_fixedTrackOverflow(t *testing.T) {
	policy := New(Config{
		Columns: []TrackDef{{Sizing: TrackFixed, Value: 20, Min: 20, Max: 20}},
		Rows:    []TrackDef{{Sizing: TrackIntrinsic}},
	})
	child := newOverflowTestChild(3, gfx.Size{W: 120, H: 40})
	layer := gfx.RectFromXYWH(0, 0, 20, 40)

	out := policy.OverflowTracks([]Child{child}, layer)
	if len(out) == 0 {
		t.Fatal("expected overflow for a fixed track narrower than its child")
	}
}

// TestOverflowTracks_emptyChildren pins the no-content case reports nothing.
func TestOverflowTracks_emptyChildren(t *testing.T) {
	policy := New(Config{
		Columns: []TrackDef{{Sizing: TrackFlex, Value: 1, Min: 0}},
		Rows:    []TrackDef{{Sizing: TrackFlex, Value: 1, Min: 0}},
	})
	if out := policy.OverflowTracks(nil, gfx.RectFromXYWH(0, 0, 40, 40)); len(out) != 0 {
		t.Fatalf("empty children reported overflow: %+v", out)
	}
}
