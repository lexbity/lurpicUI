package scale

import (
	"math"
	"testing"
)

func TestExtent_basic(t *testing.T) {
	vals := []float64{3, 1, 4, 1, 5, 9, 2, 6}
	lo, hi := Extent(vals)
	if lo != 1 || hi != 9 {
		t.Fatalf("Extent = (%f,%f), want (1,9)", lo, hi)
	}
}

func TestExtent_single_value(t *testing.T) {
	lo, hi := Extent([]float64{42})
	if lo != 42 || hi != 42 {
		t.Fatalf("Extent = (%f,%f), want (42,42)", lo, hi)
	}
}

func TestExtent_empty(t *testing.T) {
	lo, hi := Extent(nil)
	if lo != 0 || hi != 0 {
		t.Fatalf("Extent(nil) = (%f,%f), want (0,0)", lo, hi)
	}
	lo, hi = Extent([]float64{})
	if lo != 0 || hi != 0 {
		t.Fatalf("Extent([]) = (%f,%f), want (0,0)", lo, hi)
	}
}

func TestExtent_nan_skipped(t *testing.T) {
	vals := []float64{math.NaN(), 5, math.NaN(), 10, math.NaN()}
	lo, hi := Extent(vals)
	if lo != 5 || hi != 10 {
		t.Fatalf("Extent = (%f,%f), want (5,10)", lo, hi)
	}
}

func TestExtent_all_nan(t *testing.T) {
	vals := []float64{math.NaN(), math.NaN()}
	lo, hi := Extent(vals)
	if lo != 0 || hi != 0 {
		t.Fatalf("Extent(all NaN) = (%f,%f), want (0,0)", lo, hi)
	}
}

func TestExtent_negative_values(t *testing.T) {
	vals := []float64{-5, -2, -10, -1}
	lo, hi := Extent(vals)
	if lo != -10 || hi != -1 {
		t.Fatalf("Extent = (%f,%f), want (-10,-1)", lo, hi)
	}
}

func TestExtent_inf(t *testing.T) {
	vals := []float64{1, math.Inf(1), 2}
	lo, hi := Extent(vals)
	if lo != 1 || hi != math.Inf(1) {
		t.Fatalf("Extent = (%f,%f), want (1,+Inf)", lo, hi)
	}
}

// --- ExtentBy ---

func TestExtentBy_basic(t *testing.T) {
	type item struct {
		val float64
	}
	items := []item{{3}, {1}, {4}, {1}, {5}}
	lo, hi := ExtentBy(items, func(i item) float64 { return i.val })
	if lo != 1 || hi != 5 {
		t.Fatalf("ExtentBy = (%f,%f), want (1,5)", lo, hi)
	}
}

func TestExtentBy_empty(t *testing.T) {
	type item struct{ val float64 }
	lo, hi := ExtentBy([]item{}, func(i item) float64 { return i.val })
	if lo != 0 || hi != 0 {
		t.Fatalf("ExtentBy empty = (%f,%f), want (0,0)", lo, hi)
	}
}

func TestExtentBy_all_nan(t *testing.T) {
	type item struct {
		val float64
	}
	items := []item{{math.NaN()}, {math.NaN()}}
	lo, hi := ExtentBy(items, func(i item) float64 { return i.val })
	if lo != 0 || hi != 0 {
		t.Fatalf("ExtentBy all NaN = (%f,%f), want (0,0)", lo, hi)
	}
}

func TestExtentBy_nan(t *testing.T) {
	type item struct {
		val float64
	}
	items := []item{{math.NaN()}, {8}}
	lo, hi := ExtentBy(items, func(i item) float64 { return i.val })
	if lo != 8 || hi != 8 {
		t.Fatalf("ExtentBy with NaN = (%f,%f), want (8,8)", lo, hi)
	}
}

// --- NiceExtent ---

func TestNiceExtent_basic(t *testing.T) {
	vals := []float64{3, 1, 4, 1, 5, 9, 2, 6}
	lo, hi := NiceExtent(vals, 5)
	// Extent = [1, 9]. Nice(1, 9, 5):
	// tickStep(1, 9, 5) → span=8, step0=1.6, mag=1, error=1.6 → step=2
	// floor(1/2)*2=0, ceil(9/2)*2=10 → (0, 10)
	if lo != 0 || hi != 10 {
		t.Fatalf("NiceExtent = (%f,%f), want (0,10)", lo, hi)
	}
}

func TestNiceExtent_single(t *testing.T) {
	lo, hi := NiceExtent([]float64{42}, 5)
	// Single value 42 → Extent = (42,42). Expand: (37.8, 46.2)
	// Nice(37.8, 46.2, 5) → step=2, floor = 36, ceil = 48 → (36, 48)
	if lo >= hi {
		t.Fatalf("NiceExtent single = (%f,%f), expected lo < hi", lo, hi)
	}
	if lo > 42 || hi < 42 {
		t.Fatalf("NiceExtent single = (%f,%f), expected range containing 42", lo, hi)
	}
}

func TestNiceExtent_single_zero(t *testing.T) {
	lo, hi := NiceExtent([]float64{0}, 5)
	// Single value 0 → Extent = (0,0). Expand → (0, 1) then Nice
	if lo >= hi {
		t.Fatalf("NiceExtent zero = (%f,%f), expected lo < hi", lo, hi)
	}
	if lo > 0 || hi < 0 {
		t.Fatalf("NiceExtent zero = (%f,%f), expected range containing 0", lo, hi)
	}
}

func TestNiceExtent_empty(t *testing.T) {
	lo, hi := NiceExtent(nil, 5)
	if lo != 0 || hi != 0 {
		t.Fatalf("NiceExtent empty = (%f,%f), want (0,0)", lo, hi)
	}
}

func TestNiceExtent_nan_skipped(t *testing.T) {
	vals := []float64{math.NaN(), 3, math.NaN(), 7, math.NaN()}
	lo, hi := NiceExtent(vals, 5)
	// Extent = (3, 7), Nice(3, 7, 5) → tickStep(3,7,5) → step=1
	// floor(3/1)*1=3, ceil(7/1)*1=7 → (3, 7)
	if lo != 3 || hi != 7 {
		t.Fatalf("NiceExtent = (%f,%f), want (3,7)", lo, hi)
	}
}
