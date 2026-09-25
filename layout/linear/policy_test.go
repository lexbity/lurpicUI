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

// helper for weighted-fill tests: a fill child with a declared weight.
func newWeightedChild(id facet.FacetID, order int, size gfx.Size, weight float32) Child {
	c := newLinearChild(id, order, size, facet.CrossAxisStart, facet.StretchPolicy{})
	c.Attachment.Placement.Linear.MainAxisSize = facet.MainAxisMax
	c.Attachment.Placement.Linear.Weight = weight
	return c
}

func TestPolicyHorizontal_proportionalWeights(t *testing.T) {
	p := NewHorizontal(0)
	w1 := newWeightedChild(1, 0, gfx.Size{W: 10, H: 10}, 1)
	w2 := newWeightedChild(2, 1, gfx.Size{W: 10, H: 10}, 2)

	if _, err := p.Arrange([]Child{w1, w2}, gfx.RectFromXYWH(0, 0, 310, 20)); err != nil {
		t.Fatalf("Arrange: %v", err)
	}
	b1 := w1.Layout.ArrangedBounds
	b2 := w2.Layout.ArrangedBounds
	// residual 290 shared 1:2 on top of measured 10 → 106.67 / 203.33.
	if got := b1.Width(); got < 106 || got > 107 {
		t.Fatalf("weight-1 width = %v, want ~106.67", got)
	}
	if got := b2.Width(); got < 203 || got > 204 {
		t.Fatalf("weight-2 width = %v, want ~203.33", got)
	}
	if b1.Max.X != b2.Min.X {
		t.Fatalf("children must be adjacent: %#v then %#v", b1, b2)
	}
}

func TestPolicyHorizontal_zeroWeightFallbackEqualShare(t *testing.T) {
	p := NewHorizontal(0)
	w1 := newWeightedChild(1, 0, gfx.Size{W: 10, H: 10}, 0)
	w2 := newWeightedChild(2, 1, gfx.Size{W: 10, H: 10}, 0)

	if _, err := p.Arrange([]Child{w1, w2}, gfx.RectFromXYWH(0, 0, 210, 20)); err != nil {
		t.Fatalf("Arrange: %v", err)
	}
	if got := w1.Layout.ArrangedBounds.Width(); got != 105 {
		t.Fatalf("unweighted fill width = %v, want 105 (10 measured + 95 equal share)", got)
	}
	if w1.Layout.ArrangedBounds.Width() != w2.Layout.ArrangedBounds.Width() {
		t.Fatalf("equal shares diverged: %v vs %v",
			w1.Layout.ArrangedBounds.Width(), w2.Layout.ArrangedBounds.Width())
	}
}

func TestPolicyHorizontal_negativeWeightClampsToEqualShare(t *testing.T) {
	p := NewHorizontal(0)
	w1 := newWeightedChild(1, 0, gfx.Size{W: 10, H: 10}, -5)
	w2 := newWeightedChild(2, 1, gfx.Size{W: 10, H: 10}, 0)

	if _, err := p.Arrange([]Child{w1, w2}, gfx.RectFromXYWH(0, 0, 210, 20)); err != nil {
		t.Fatalf("Arrange: %v", err)
	}
	if w1.Layout.ArrangedBounds.Width() != w2.Layout.ArrangedBounds.Width() {
		t.Fatalf("negative weight must clamp to equal share: %v vs %v",
			w1.Layout.ArrangedBounds.Width(), w2.Layout.ArrangedBounds.Width())
	}
}

func TestPolicyVertical_proportionalWeights(t *testing.T) {
	p := NewVertical(0)
	w1 := newWeightedChild(1, 0, gfx.Size{W: 10, H: 10}, 1)
	w2 := newWeightedChild(2, 1, gfx.Size{W: 10, H: 10}, 3)

	if _, err := p.Arrange([]Child{w1, w2}, gfx.RectFromXYWH(0, 0, 20, 410)); err != nil {
		t.Fatalf("Arrange: %v", err)
	}
	h1 := w1.Layout.ArrangedBounds.Height()
	h2 := w2.Layout.ArrangedBounds.Height()
	// residual 390 shared 1:3 on top of measured 10 → 107.5 / 302.5.
	if h1 < 107 || h1 > 108 {
		t.Fatalf("weight-1 height = %v, want ~107.5", h1)
	}
	if h2 < 302 || h2 > 303 {
		t.Fatalf("weight-3 height = %v, want ~302.5", h2)
	}
}

func TestPolicyHorizontal_weightedAndUnweightedMix(t *testing.T) {
	p := NewHorizontal(0)
	a := newWeightedChild(1, 0, gfx.Size{W: 10, H: 10}, 1) // weighted
	b := newWeightedChild(2, 1, gfx.Size{W: 10, H: 10}, 0) // unweighted fill
	fixed := newLinearChild(3, 2, gfx.Size{W: 50, H: 10}, facet.CrossAxisStart, facet.StretchPolicy{})

	if _, err := p.Arrange([]Child{a, b, fixed}, gfx.RectFromXYWH(0, 0, 300, 20)); err != nil {
		t.Fatalf("Arrange: %v", err)
	}
	// residual = 300 - 70 = 230. Weighted gets all of it (10 measured + 230);
	// unweighted gets the zero remainder.
	if got := a.Layout.ArrangedBounds.Width(); got < 239 || got > 241 {
		t.Fatalf("weighted child width = %v, want ~240", got)
	}
	if got := b.Layout.ArrangedBounds.Width(); got != 10 {
		t.Fatalf("unweighted fill child width = %v, want 10 (measured only; weighted child consumed the residual)", got)
	}
	if got := fixed.Layout.ArrangedBounds.Width(); got != 50 {
		t.Fatalf("fixed child width = %v, want 50", got)
	}
}
