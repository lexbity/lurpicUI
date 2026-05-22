package radial

import (
	"math"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

func TestRadialLayoutMathEvenDistribution(t *testing.T) {
	policy := New(Config{
		DefaultRadius:    50,
		StartAngle:       0,
		WritingDirection: facet.WritingDirectionLTR,
	})

	children := []Child{
		newTestChild(1, gfx.Size{W: 10, H: 10}, facet.RadialPlacement{Angle: math.NaN()}, facet.SupportsRadial),
		newTestChild(2, gfx.Size{W: 10, H: 10}, facet.RadialPlacement{Angle: math.NaN()}, facet.SupportsRadial),
		newTestChild(3, gfx.Size{W: 10, H: 10}, facet.RadialPlacement{Angle: math.NaN()}, facet.SupportsRadial),
		newTestChild(4, gfx.Size{W: 10, H: 10}, facet.RadialPlacement{Angle: math.NaN()}, facet.SupportsRadial),
	}

	measure, err := policy.Measure(facet.MeasureContext{}, children, gfx.Size{W: 200, H: 200})
	if err != nil {
		t.Fatalf("Measure: %v", err)
	}
	if !nearFloat(measure.W, 110, 0.001) || !nearFloat(measure.H, 110, 0.001) {
		t.Fatalf("Measure = %#v, want 110x110", measure)
	}

	arranged, err := policy.Arrange(facet.ArrangeContext{}, children, gfx.RectFromXYWH(0, 0, 200, 200))
	if err != nil {
		t.Fatalf("Arrange: %v", err)
	}
	if len(arranged) != 4 {
		t.Fatalf("arranged count = %d, want 4", len(arranged))
	}

	wantAngles := []float64{0, math.Pi / 2, math.Pi, 3 * math.Pi / 2}
	wantCenters := []gfx.Point{
		{X: 150, Y: 100},
		{X: 100, Y: 150},
		{X: 50, Y: 100},
		{X: 100, Y: 50},
	}
	for i := range arranged {
		if !nearFloat(float32(arranged[i].Angle), float32(wantAngles[i]), 0.001) {
			t.Fatalf("child %d angle = %.6f, want %.6f", i, arranged[i].Angle, wantAngles[i])
		}
		if !nearFloat(arranged[i].Center.X, wantCenters[i].X, 0.001) || !nearFloat(arranged[i].Center.Y, wantCenters[i].Y, 0.001) {
			t.Fatalf("child %d center = %#v, want %#v", i, arranged[i].Center, wantCenters[i])
		}
	}
}

func TestRadialLayoutRTL(t *testing.T) {
	policy := New(Config{
		DefaultRadius:    50,
		StartAngle:       0,
		WritingDirection: facet.WritingDirectionRTL,
	})

	children := []Child{
		newTestChild(1, gfx.Size{W: 10, H: 10}, facet.RadialPlacement{Angle: math.NaN()}, facet.SupportsRadial),
		newTestChild(2, gfx.Size{W: 10, H: 10}, facet.RadialPlacement{Angle: math.NaN()}, facet.SupportsRadial),
		newTestChild(3, gfx.Size{W: 10, H: 10}, facet.RadialPlacement{Angle: math.NaN()}, facet.SupportsRadial),
		newTestChild(4, gfx.Size{W: 10, H: 10}, facet.RadialPlacement{Angle: math.NaN()}, facet.SupportsRadial),
	}

	arranged, err := policy.Arrange(facet.ArrangeContext{}, children, gfx.RectFromXYWH(0, 0, 200, 200))
	if err != nil {
		t.Fatalf("Arrange: %v", err)
	}

	wantAngles := []float64{0, -math.Pi / 2, -math.Pi, -3 * math.Pi / 2}
	for i := range arranged {
		if !nearFloat(float32(arranged[i].Angle), float32(wantAngles[i]), 0.001) {
			t.Fatalf("child %d angle = %.6f, want %.6f", i, arranged[i].Angle, wantAngles[i])
		}
	}
}

func TestRadialLayoutClipsAndSizes(t *testing.T) {
	var seen facet.Constraints
	role := &facet.LayoutRole{
		OnMeasure: func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
			seen = c
			return facet.MeasureResult{Size: gfx.Size{W: 18, H: 24}}
		},
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {},
	}
	role.Child.SupportedPlacement = facet.SupportsRadial

	idFacet := facet.NewFacet()
	child := Child{
		FacetID:    idFacet.ID(),
		Attachment: facet.Attachment{Placement: facet.Placement{Mode: facet.PlacementRadial, Radial: facet.RadialPlacement{Angle: 0, RadiusTrack: -1}}},
		Layout:     role,
		Contract:   role.Child,
	}
	policy := New(Config{
		DefaultRadius:    30,
		StartAngle:       0,
		WritingDirection: facet.WritingDirectionLTR,
	})

	measure, err := policy.Measure(facet.MeasureContext{}, []Child{child}, gfx.Size{W: 80, H: 60})
	if err != nil {
		t.Fatalf("Measure: %v", err)
	}
	if seen.MaxSize != (gfx.Size{W: 80, H: 60}) {
		t.Fatalf("constraints = %#v, want MaxSize 80x60", seen)
	}
	if !nearFloat(measure.W, 78, 0.001) || !nearFloat(measure.H, 24, 0.001) {
		t.Fatalf("Measure = %#v, want 78x24", measure)
	}

	arranged, err := policy.Arrange(facet.ArrangeContext{}, []Child{child}, gfx.RectFromXYWH(0, 0, measure.W, measure.H))
	if err != nil {
		t.Fatalf("Arrange: %v", err)
	}
	if len(arranged) != 1 {
		t.Fatalf("arranged count = %d, want 1", len(arranged))
	}
	if arranged[0].Bounds != (gfx.RectFromXYWH(60, 0, 18, 24)) {
		t.Fatalf("arranged bounds = %#v, want within measured box", arranged[0].Bounds)
	}
}

func newTestChild(id facet.FacetID, size gfx.Size, placement facet.RadialPlacement, supported facet.PlacementModeSet) Child {
	if placement.RadiusTrack == 0 {
		placement.RadiusTrack = -1
	}
	role := &facet.LayoutRole{}
	role.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: size}
	}
	role.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		role.ArrangedBounds = bounds
	}
	role.Child.SupportedPlacement = supported
	return Child{
		FacetID:    id,
		Attachment: facet.Attachment{Placement: facet.Placement{Mode: facet.PlacementRadial, Radial: placement}},
		Layout:     role,
		Contract:   role.Child,
	}
}

func nearFloat(a, b, tol float32) bool {
	if a > b {
		return a-b <= tol
	}
	return b-a <= tol
}
