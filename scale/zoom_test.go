package scale

import (
	"math"
	"testing"
)

// --- PanDomain ---

func TestPanDomain_basic(t *testing.T) {
	lo, hi := PanDomain(10, 100, 5)
	if lo != 15 || hi != 105 {
		t.Fatalf("PanDomain(10,100,5) = (%f,%f), want (15,105)", lo, hi)
	}
}

func TestPanDomain_reversible(t *testing.T) {
	lo, hi := PanDomain(10, 100, 50)
	lo, hi = PanDomain(lo, hi, -50)
	if lo != 10 || hi != 100 {
		t.Fatalf("reversed = (%f,%f), want (10,100)", lo, hi)
	}
}

func TestPanDomain_additivity(t *testing.T) {
	// Pan by 10 then by 20 should equal pan by 30
	lo, hi := PanDomain(10, 100, 10)
	lo, hi = PanDomain(lo, hi, 20)
	wantLo, wantHi := PanDomain(10, 100, 30)
	if lo != wantLo || hi != wantHi {
		t.Fatalf("additive pan = (%f,%f), want (%f,%f)", lo, hi, wantLo, wantHi)
	}
}

func TestPanDomain_zero_delta(t *testing.T) {
	lo, hi := PanDomain(10, 100, 0)
	if lo != 10 || hi != 100 {
		t.Fatalf("PanDomain(10,100,0) = (%f,%f), want (10,100)", lo, hi)
	}
}

func TestPanDomain_negative_delta(t *testing.T) {
	lo, hi := PanDomain(10, 100, -20)
	if lo != -10 || hi != 80 {
		t.Fatalf("PanDomain(10,100,-20) = (%f,%f), want (-10,80)", lo, hi)
	}
}

func TestPanDomain_degenerate(t *testing.T) {
	lo, hi := PanDomain(5, 5, 10)
	if lo != 15 || hi != 15 {
		t.Fatalf("PanDomain(5,5,10) = (%f,%f), want (15,15)", lo, hi)
	}
}

// --- ZoomDomain ---

func TestZoomDomain_zoom_in(t *testing.T) {
	lo, hi := ZoomDomain(0, 100, 50, 2)
	// focal=50, factor=2: lo'=50-(50-0)/2=25, hi'=50+(100-50)/2=75
	if lo != 25 || hi != 75 {
		t.Fatalf("ZoomDomain(0,100,50,2) = (%f,%f), want (25,75)", lo, hi)
	}
}

func TestZoomDomain_zoom_out(t *testing.T) {
	lo, hi := ZoomDomain(25, 75, 50, 0.5)
	// factor=0.5: lo'=50-(50-25)/0.5=50-50=0, hi'=50+(75-50)/0.5=50+50=100
	if lo != 0 || hi != 100 {
		t.Fatalf("ZoomDomain(25,75,50,0.5) = (%f,%f), want (0,100)", lo, hi)
	}
}

func TestZoomDomain_focal_preserved_in_linear_scale(t *testing.T) {
	// When the zoomed domain is used to construct a linear scale, the focal
	// point should map to the same range position.
	lo, hi := 0.0, 100.0
	focal := 40.0
	s := NewLinear(WithDomain(lo, hi), WithRange(0, 500))
	focalPos := s.Map(focal)

	// Zoom in 3x around the focal
	newLo, newHi := ZoomDomain(lo, hi, focal, 3)
	s2 := NewLinear(WithDomain(newLo, newHi), WithRange(0, 500))
	newFocalPos := s2.Map(focal)

	if math.Abs(newFocalPos-focalPos) > 1e-12 {
		t.Fatalf("focal position changed: %.10f -> %.10f", focalPos, newFocalPos)
	}
}

func TestZoomDomain_asymmetric_focal(t *testing.T) {
	lo, hi := ZoomDomain(0, 200, 20, 4)
	// focal=20, factor=4: lo'=20-(20-0)/4=15, hi'=20+(200-20)/4=65
	expectedLo := 20.0 - (20.0-0.0)/4.0   // 15
	expectedHi := 20.0 + (200.0-20.0)/4.0 // 65
	if lo != expectedLo || hi != expectedHi {
		t.Fatalf("ZoomDomain(0,200,20,4) = (%f,%f), want (%f,%f)", lo, hi, expectedLo, expectedHi)
	}
}

func TestZoomDomain_factor_one(t *testing.T) {
	lo, hi := ZoomDomain(0, 100, 50, 1)
	if lo != 0 || hi != 100 {
		t.Fatalf("ZoomDomain(0,100,50,1) = (%f,%f), want (0,100)", lo, hi)
	}
}

func TestZoomDomain_degenerate_domain(t *testing.T) {
	lo, hi := ZoomDomain(5, 5, 0, 2)
	if lo != 5 || hi != 5 {
		t.Fatalf("ZoomDomain(5,5,0,2) = (%f,%f), want (5,5)", lo, hi)
	}
}

func TestZoomDomain_non_positive_factor(t *testing.T) {
	lo, hi := ZoomDomain(0, 100, 50, 0)
	if lo != 0 || hi != 100 {
		t.Fatalf("ZoomDomain(0,100,50,0) = (%f,%f), want (0,100)", lo, hi)
	}
	lo, hi = ZoomDomain(0, 100, 50, -2)
	if lo != 0 || hi != 100 {
		t.Fatalf("ZoomDomain(0,100,50,-2) = (%f,%f), want (0,100)", lo, hi)
	}
}

func TestZoomDomain_nan_inputs(t *testing.T) {
	lo, hi := ZoomDomain(math.NaN(), 100, 50, 2)
	if !math.IsNaN(lo) || hi != 100 {
		t.Fatalf("expected NaN lo, got (%f,%f)", lo, hi)
	}
	lo, hi = ZoomDomain(0, 100, math.NaN(), 2)
	if lo != 0 || hi != 100 {
		t.Fatalf("expected unchanged with NaN focal, got (%f,%f)", lo, hi)
	}
}

func TestZoomDomain_focal_outside_domain(t *testing.T) {
	// Focal outside the domain: the math still works
	lo, hi := ZoomDomain(0, 100, 200, 2)
	// focal=200, lo'=200-(200-0)/2=100, hi'=200+(100-200)/2=150
	if lo != 100 || hi != 150 {
		t.Fatalf("ZoomDomain(0,100,200,2) = (%f,%f), want (100,150)", lo, hi)
	}
}

func TestZoomDomain_wide_range(t *testing.T) {
	// Test with very large range to ensure numeric stability
	lo, hi := ZoomDomain(-1e10, 1e10, 0, 10)
	wantSpan := 2e10 / 10 // 2e9
	gotSpan := hi - lo
	if math.Abs(gotSpan-wantSpan) > 1 {
		t.Fatalf("ZoomDomain span = %f, want ~%f", gotSpan, wantSpan)
	}
}

func TestZoomDomain_extreme_factor(t *testing.T) {
	lo, hi := ZoomDomain(0, 100, 50, 1e6)
	// Very large factor: domain should be essentially at the focal
	if lo >= 50 || hi <= 50 {
		t.Fatalf("ZoomDomain extreme factor = (%f,%f), expected focal inside", lo, hi)
	}
}

// --- Pan + Zoom combined ---

func TestPanAndZoom_round_trip(t *testing.T) {
	lo, hi := 0.0, 100.0
	// Pan by 50
	pLo, pHi := PanDomain(lo, hi, 50)
	// Zoom in 2x around center
	zLo, zHi := ZoomDomain(pLo, pHi, 75, 2)
	// Zoom out 2x back
	uzLo, uzHi := ZoomDomain(zLo, zHi, 75, 0.5)
	// Pan back
	finalLo, finalHi := PanDomain(uzLo, uzHi, -50)
	if math.Abs(finalLo-lo) > 1e-10 || math.Abs(finalHi-hi) > 1e-10 {
		t.Fatalf("round-trip = (%f,%f), want (%f,%f)", finalLo, finalHi, lo, hi)
	}
}

// --- benchmarks ---

func BenchmarkPanDomain(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		PanDomain(0, 100, float64(i%200-100))
	}
}

func BenchmarkZoomDomain(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ZoomDomain(0, 100, 50, float64(i%10+1))
	}
}
