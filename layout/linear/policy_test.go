package linear

import (
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

func newLinearChild(id facet.FacetID, order int, size gfx.Size, cross facet.CrossAxisAlignment, stretch facet.StretchPolicy) Child {
	role := &facet.LayoutRole{}
	role.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: size}
	}
	role.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		role.ArrangedBounds = bounds
	}
	role.Child.SupportedPlacement = facet.SupportsLinear
	role.Child.Stretch = stretch
	return Child{
		FacetID: id,
		Attachment: facet.Attachment{
			Placement: facet.Placement{
				Mode: facet.PlacementLinear,
				Linear: facet.LinearPlacement{
					Order:          order,
					CrossAxisAlign: cross,
					MainAxisSize:   facet.MainAxisAuto,
				},
			},
		},
		Layout:   role,
		Contract: role.Child,
	}
}

func TestPolicyHorizontal_order_gap_and_stretch(t *testing.T) {
	p := NewHorizontal(10)
	stretched := newLinearChild(2, 0, gfx.Size{W: 20, H: 10}, facet.CrossAxisStretch, facet.StretchPolicy{
		Width:  facet.StretchAlways,
		Height: facet.StretchAlways,
	})
	fixed := newLinearChild(1, 1, gfx.Size{W: 50, H: 20}, facet.CrossAxisEnd, facet.StretchPolicy{})

	size, err := p.Measure([]Child{fixed, stretched}, gfx.Size{W: 200, H: 50})
	if err != nil {
		t.Fatalf("Measure: %v", err)
	}
	if size != (gfx.Size{W: 80, H: 20}) {
		t.Fatalf("measure = %#v", size)
	}

	arranged, err := p.Arrange([]Child{fixed, stretched}, gfx.RectFromXYWH(0, 0, 200, 50))
	if err != nil {
		t.Fatalf("Arrange: %v", err)
	}
	byID := map[facet.FacetID]ArrangedChild{}
	for i := range arranged {
		byID[arranged[i].FacetID] = arranged[i]
	}
	if got := byID[2].Bounds; got != (gfx.RectFromXYWH(0, 0, 140, 50)) {
		t.Fatalf("stretched child bounds = %#v", got)
	}
	if got := byID[1].Bounds; got != (gfx.RectFromXYWH(150, 30, 50, 20)) {
		t.Fatalf("fixed child bounds = %#v", got)
	}
}

func TestPolicyVertical_order_gap_and_stretch(t *testing.T) {
	p := NewVertical(8)
	stretched := newLinearChild(4, 0, gfx.Size{W: 10, H: 20}, facet.CrossAxisStretch, facet.StretchPolicy{
		Width:  facet.StretchAlways,
		Height: facet.StretchAlways,
	})
	fixed := newLinearChild(3, 1, gfx.Size{W: 20, H: 40}, facet.CrossAxisCenter, facet.StretchPolicy{})

	arranged, err := p.Arrange([]Child{fixed, stretched}, gfx.RectFromXYWH(0, 0, 100, 200))
	if err != nil {
		t.Fatalf("Arrange: %v", err)
	}
	byID := map[facet.FacetID]ArrangedChild{}
	for i := range arranged {
		byID[arranged[i].FacetID] = arranged[i]
	}
	if got := byID[4].Bounds; got != (gfx.RectFromXYWH(0, 0, 100, 152)) {
		t.Fatalf("stretched child bounds = %#v", got)
	}
	if got := byID[3].Bounds; got != (gfx.RectFromXYWH(40, 160, 20, 40)) {
		t.Fatalf("fixed child bounds = %#v", got)
	}
}

func TestPolicy_rejectsBaselineAlignment(t *testing.T) {
	p := NewHorizontal(0)
	child := newLinearChild(1, 0, gfx.Size{W: 10, H: 10}, facet.CrossAxisBaseline, facet.StretchPolicy{})
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for baseline alignment")
		} else if msg, ok := r.(string); !ok || !strings.Contains(msg, "layout contract violation") || !strings.Contains(msg, "baseline alignment not supported") {
			t.Fatalf("panic = %#v", r)
		}
	}()
	_, _ = p.Arrange([]Child{child}, gfx.RectFromXYWH(0, 0, 100, 100))
}
