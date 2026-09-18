package facet

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
)

// TestLayoutRoleGrowClamp_countsAndClamps pins RX-1 Q5 / NFR-8: arranging a
// Grow mark below its measured size clamps to that size and increments the
// frame's OverflowClampedCount. Empty bounds (a host hiding the facet) are
// exempt so a hidden Grow facet stays hidden.
func TestLayoutRoleGrowClamp_countsAndClamps(t *testing.T) {
	role := &LayoutRole{}
	role.Parent.Overflow = OverflowGrow
	role.MeasuredSize = gfx.Size{W: 200, H: 100}

	before := DrainOverflowClamped()
	role.Arrange(ArrangeContext{}, gfx.RectFromXYWH(0, 0, 40, 30))
	if role.ArrangedBounds.Width() != 200 || role.ArrangedBounds.Height() != 100 {
		t.Fatalf("clamped bounds = %v, want 200x100", role.ArrangedBounds)
	}
	if DrainOverflowClamped()-before != 1 {
		t.Fatalf("OverflowClampedCount delta = %d, want 1", DrainOverflowClamped()-before)
	}
}

// TestLayoutRoleGrowClamp_exemptsEmptyBounds pins that arranging a Grow facet
// to empty bounds (host hiding it) neither clamps nor counts.
func TestLayoutRoleGrowClamp_exemptsEmptyBounds(t *testing.T) {
	role := &LayoutRole{}
	role.Parent.Overflow = OverflowGrow
	role.MeasuredSize = gfx.Size{W: 200, H: 100}

	before := DrainOverflowClamped()
	role.Arrange(ArrangeContext{}, gfx.Rect{})
	if !role.ArrangedBounds.IsEmpty() {
		t.Fatalf("empty arrangement was clamped: %v", role.ArrangedBounds)
	}
	if DrainOverflowClamped()-before != 0 {
		t.Fatalf("empty arrangement counted an overflow clamp")
	}
}

// TestLayoutRoleGrowClamp_noCountWhenFits pins the happy path: arranging at or
// above the measured size never clamps.
func TestLayoutRoleGrowClamp_noCountWhenFits(t *testing.T) {
	role := &LayoutRole{}
	role.Parent.Overflow = OverflowGrow
	role.MeasuredSize = gfx.Size{W: 200, H: 100}

	before := DrainOverflowClamped()
	role.Arrange(ArrangeContext{}, gfx.RectFromXYWH(0, 0, 240, 120))
	if role.ArrangedBounds.Width() != 240 || role.ArrangedBounds.Height() != 120 {
		t.Fatalf("fits arrangement was clamped: %v", role.ArrangedBounds)
	}
	if DrainOverflowClamped()-before != 0 {
		t.Fatalf("fits arrangement counted an overflow clamp")
	}
}
