package linear

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

func newOverflowLinearChild(id facet.FacetID, size gfx.Size) Child {
	role := &facet.LayoutRole{}
	role.MeasuredSize = size
	return Child{
		FacetID: id,
		Layout:  role,
		Attachment: facet.Attachment{
			Placement: facet.Placement{Mode: facet.PlacementLinear, Linear: facet.LinearPlacement{Order: 0}},
		},
		Contract: facet.GroupChildContract{SupportedPlacement: facet.SupportsLinear},
	}
}

// TestMainAxisOverflow_reportsContentBeyondBounds pins RX-1 FR-5 linear
// per-axis overflow reporting: total content extent vs the arranged main axis.
func TestMainAxisOverflow_reportsContentBeyondBounds(t *testing.T) {
	policy := New(Config{Axis: Vertical, Gap: 4})
	children := []Child{
		newOverflowLinearChild(1, gfx.Size{W: 100, H: 50}),
		newOverflowLinearChild(2, gfx.Size{W: 100, H: 70}),
	}
	bounds := gfx.RectFromXYWH(0, 0, 100, 60)

	out := policy.MainAxisOverflow(children, bounds)
	wantNeed := float32(50 + 4 + 70)
	if out.Need != wantNeed {
		t.Fatalf("need = %v, want %v", out.Need, wantNeed)
	}
	if out.Size != 60 {
		t.Fatalf("size = %v, want 60", out.Size)
	}
	if out.Need <= out.Size {
		t.Fatalf("expected overflow (need %v > size %v)", out.Need, out.Size)
	}
}

// TestMainAxisOverflow_fitsWithinBounds pins the no-overflow case.
func TestMainAxisOverflow_fitsWithinBounds(t *testing.T) {
	policy := New(Config{Axis: Horizontal, Gap: 0})
	children := []Child{
		newOverflowLinearChild(1, gfx.Size{W: 30, H: 20}),
		newOverflowLinearChild(2, gfx.Size{W: 30, H: 20}),
	}
	bounds := gfx.RectFromXYWH(0, 0, 200, 100)

	out := policy.MainAxisOverflow(children, bounds)
	if out.Need > out.Size {
		t.Fatalf("unexpected overflow: need %v > size %v", out.Need, out.Size)
	}
}
