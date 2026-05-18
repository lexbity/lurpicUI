package anchor

import (
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

type mapCache map[facet.AnchorID]gfx.Point

func (m mapCache) Get(id facet.AnchorID) (gfx.Point, bool) {
	pos, ok := m[id]
	return pos, ok
}

func newAnchorChild(id facet.FacetID, placement facet.AnchorPlacement, size gfx.Size) Child {
	role := &facet.LayoutRole{}
	role.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: size}
	}
	role.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		role.ArrangedBounds = bounds
	}
	role.Child.SupportedPlacement = facet.SupportsAnchor
	return Child{
		FacetID: id,
		Attachment: facet.Attachment{
			Placement: facet.Placement{Mode: facet.PlacementAnchor, Anchor: placement},
		},
		Layout:   role,
		Contract: role.Child,
	}
}

func TestPolicyArrange_positions_children_by_side(t *testing.T) {
	cache := mapCache{
		"mark": {X: 100, Y: 200},
	}
	cases := []struct {
		name string
		side facet.AnchorSide
		want gfx.Rect
	}{
		{name: "above", side: facet.AnchorAbove, want: gfx.RectFromXYWH(75, 162, 50, 30)},
		{name: "below", side: facet.AnchorBelow, want: gfx.RectFromXYWH(75, 208, 50, 30)},
		{name: "left", side: facet.AnchorLeft, want: gfx.RectFromXYWH(42, 185, 50, 30)},
		{name: "right", side: facet.AnchorRight, want: gfx.RectFromXYWH(108, 185, 50, 30)},
		{name: "center", side: facet.AnchorCenter, want: gfx.RectFromXYWH(75, 185, 50, 30)},
	}
	p := New()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			child := newAnchorChild(1, facet.AnchorPlacement{AnchorRef: "mark", Side: tc.side, Gap: facet.ResolvedScalar(8)}, gfx.Size{W: 50, H: 30})
			arranged, err := p.Arrange([]Child{child}, gfx.RectFromXYWH(0, 0, 300, 300), cache, false)
			if err != nil {
				t.Fatalf("Arrange: %v", err)
			}
			if len(arranged) != 1 {
				t.Fatalf("arranged count = %d", len(arranged))
			}
			if arranged[0].Bounds != tc.want {
				t.Fatalf("bounds = %#v, want %#v", arranged[0].Bounds, tc.want)
			}
		})
	}
}

func TestPolicyArrange_panics_on_missing_anchor(t *testing.T) {
	cache := mapCache{}
	child := newAnchorChild(1, facet.AnchorPlacement{AnchorRef: "missing", Side: facet.AnchorCenter}, gfx.Size{W: 20, H: 10})
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for missing anchor")
		} else if msg, ok := r.(string); !ok || !strings.Contains(msg, "layout contract violation") || !strings.Contains(msg, "anchor \"missing\"") {
			t.Fatalf("panic = %#v", r)
		}
	}()
	_, _ = New().Arrange([]Child{child}, gfx.RectFromXYWH(0, 0, 100, 100), cache, false)
}

func TestPolicyArrange_allows_overflow_when_enabled(t *testing.T) {
	cache := mapCache{
		"edge": {X: 50, Y: 5},
	}
	child := newAnchorChild(1, facet.AnchorPlacement{AnchorRef: "edge", Side: facet.AnchorAbove, Gap: facet.ResolvedScalar(10)}, gfx.Size{W: 20, H: 20})
	arranged, err := New().Arrange([]Child{child}, gfx.RectFromXYWH(0, 0, 100, 100), cache, true)
	if err != nil {
		t.Fatalf("Arrange: %v", err)
	}
	if arranged[0].Bounds != gfx.RectFromXYWH(40, -25, 20, 20) {
		t.Fatalf("overflow bounds = %#v", arranged[0].Bounds)
	}
}

func TestPolicyArrange_clamps_to_bounds(t *testing.T) {
	cache := mapCache{
		"edge": {X: 50, Y: 5},
	}
	child := newAnchorChild(1, facet.AnchorPlacement{AnchorRef: "edge", Side: facet.AnchorAbove, Gap: facet.ResolvedScalar(10)}, gfx.Size{W: 20, H: 20})
	arranged, err := New().Arrange([]Child{child}, gfx.RectFromXYWH(0, 0, 100, 100), cache, false)
	if err != nil {
		t.Fatalf("Arrange: %v", err)
	}
	if arranged[0].Bounds != gfx.RectFromXYWH(40, 0, 20, 20) {
		t.Fatalf("clamped bounds = %#v", arranged[0].Bounds)
	}
}
