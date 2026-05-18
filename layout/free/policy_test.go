package free

import (
	"math"
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

func newChild(id facet.FacetID, placement facet.FreePlacement, size gfx.Size) Child {
	role := &facet.LayoutRole{}
	role.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: size}
	}
	role.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		role.ArrangedBounds = bounds
	}
	role.Child.SupportedPlacement = facet.SupportsFree
	return Child{
		FacetID: id,
		Attachment: facet.Attachment{
			Placement: facet.Placement{
				Mode: facet.PlacementFree,
				Free: placement,
			},
		},
		Layout:   role,
		Contract: role.Child,
	}
}

func TestPolicyArrange_usesExactCoordinates(t *testing.T) {
	p := New()
	child := newChild(1, facet.FreePlacement{X: facet.ResolvedScalar(12), Y: facet.ResolvedScalar(34)}, gfx.Size{W: 30, H: 40})
	arranged, err := p.Arrange([]Child{child}, gfx.RectFromXYWH(10, 20, 100, 100), false)
	if err != nil {
		t.Fatalf("Arrange: %v", err)
	}
	if arranged[0].Bounds != (gfx.RectFromXYWH(22, 54, 30, 40)) {
		t.Fatalf("bounds = %#v", arranged[0].Bounds)
	}
}

func TestPolicyArrange_usesOptionalSize(t *testing.T) {
	p := New()
	child := newChild(1, facet.FreePlacement{
		X:      facet.ResolvedScalar(5),
		Y:      facet.ResolvedScalar(7),
		Width:  facet.OptionalScalar(80),
		Height: facet.OptionalScalar(60),
	}, gfx.Size{W: 30, H: 40})
	arranged, err := p.Arrange([]Child{child}, gfx.RectFromXYWH(0, 0, 200, 200), false)
	if err != nil {
		t.Fatalf("Arrange: %v", err)
	}
	if arranged[0].Bounds != (gfx.RectFromXYWH(5, 7, 80, 60)) {
		t.Fatalf("bounds = %#v", arranged[0].Bounds)
	}
}

func TestPolicyArrange_clampsWhenOverflowDisabled(t *testing.T) {
	p := New()
	child := newChild(1, facet.FreePlacement{X: facet.ResolvedScalar(-20), Y: facet.ResolvedScalar(-10)}, gfx.Size{W: 30, H: 40})
	arranged, err := p.Arrange([]Child{child}, gfx.RectFromXYWH(0, 0, 50, 50), false)
	if err != nil {
		t.Fatalf("Arrange: %v", err)
	}
	if arranged[0].Bounds.Min != (gfx.Point{}) {
		t.Fatalf("bounds = %#v", arranged[0].Bounds)
	}
}

func TestPolicyArrange_allowsOverflowWhenEnabled(t *testing.T) {
	p := New()
	child := newChild(1, facet.FreePlacement{X: facet.ResolvedScalar(-20), Y: facet.ResolvedScalar(-10)}, gfx.Size{W: 30, H: 40})
	arranged, err := p.Arrange([]Child{child}, gfx.RectFromXYWH(0, 0, 50, 50), true)
	if err != nil {
		t.Fatalf("Arrange: %v", err)
	}
	if arranged[0].Bounds.Min != (gfx.Point{X: -20, Y: -10}) {
		t.Fatalf("bounds = %#v", arranged[0].Bounds)
	}
}

func TestPolicyArrange_panicsOnNonFiniteCoordinates(t *testing.T) {
	p := New()
	child := newChild(1, facet.FreePlacement{
		X: facet.ResolvedScalar(float32(math.NaN())),
		Y: facet.ResolvedScalar(0),
	}, gfx.Size{W: 10, H: 10})
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for NaN coordinate")
		} else if msg, ok := r.(string); !ok || !strings.Contains(msg, "layout contract violation") || !strings.Contains(msg, "free placement requires finite coordinates") {
			t.Fatalf("panic = %#v", r)
		}
	}()
	_, _ = p.Arrange([]Child{child}, gfx.RectFromXYWH(0, 0, 100, 100), false)
}

func TestPolicyArrange_panicsOnNonFiniteSize(t *testing.T) {
	p := New()
	child := newChild(1, facet.FreePlacement{
		X:      facet.ResolvedScalar(0),
		Y:      facet.ResolvedScalar(0),
		Width:  facet.ResolvedOptionalScalar{Valid: true, Value: facet.ResolvedScalar(float32(math.Inf(1)))},
		Height: facet.OptionalScalar(10),
	}, gfx.Size{W: 10, H: 10})
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for infinite width")
		} else if msg, ok := r.(string); !ok || !strings.Contains(msg, "layout contract violation") || !strings.Contains(msg, "free placement width must be finite") {
			t.Fatalf("panic = %#v", r)
		}
	}()
	_, _ = p.Arrange([]Child{child}, gfx.RectFromXYWH(0, 0, 100, 100), false)
}
