package facet

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
)

func TestLayoutRole_measure_and_arrange_record_layer_local_bounds(t *testing.T) {
	role := &LayoutRole{
		OnMeasure: func(c Constraints) gfx.Size {
			if c.MinSize != (gfx.Size{}) {
				t.Fatalf("unexpected min size: %#v", c.MinSize)
			}
			return gfx.Size{W: 40, H: 24}
		},
		OnArrange: func(bounds gfx.Rect) {
			if bounds != (gfx.RectFromXYWH(10, 20, 40, 24)) {
				t.Fatalf("unexpected arrange bounds: %#v", bounds)
			}
		},
	}

	got := role.Measure(Constraints{})
	if got != (gfx.Size{W: 40, H: 24}) {
		t.Fatalf("Measure = %#v, want measured size", got)
	}
	role.Arrange(gfx.RectFromXYWH(10, 20, 40, 24))

	if role.MeasuredSize != (gfx.Size{W: 40, H: 24}) {
		t.Fatalf("MeasuredSize = %#v, want cached measurement", role.MeasuredSize)
	}
	if role.ArrangedBounds != (gfx.RectFromXYWH(10, 20, 40, 24)) {
		t.Fatalf("ArrangedBounds = %#v, want arranged bounds", role.ArrangedBounds)
	}
}
