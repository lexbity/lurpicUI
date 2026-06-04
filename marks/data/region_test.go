package data

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/store"
)

func TestRegionFromBounds_sets_lo_hi_from_width(t *testing.T) {
	rng := store.NewValueStore([2]float64{0, 100})
	bounds := gfx.RectFromXYWH(10, 20, 400, 300)

	RegionFromBounds(rng, bounds)

	got := rng.Get()
	if got[0] != 0 || got[1] != 400 {
		t.Fatalf("range = [%f,%f], want [0,400]", got[0], got[1])
	}
}

func TestRegionFromBounds_zero_width_range(t *testing.T) {
	rng := store.NewValueStore([2]float64{0, 100})
	bounds := gfx.Rect{}

	RegionFromBounds(rng, bounds)

	got := rng.Get()
	if got[0] != 0 || got[1] != 0 {
		t.Fatalf("range = [%f,%f], want [0,0]", got[0], got[1])
	}
}

func TestRegionFromBounds_nil_store_noop(t *testing.T) {
	bounds := gfx.RectFromXYWH(0, 0, 800, 600)
	RegionFromBounds(nil, bounds)
}

func TestRegionFromBounds_version_bumps(t *testing.T) {
	rng := store.NewValueStore([2]float64{0, 100})
	v0 := rng.Version()

	RegionFromBounds(rng, gfx.RectFromXYWH(0, 0, 800, 600))

	v1 := rng.Version()
	if v1 <= v0 {
		t.Fatal("version must bump after RegionFromBounds")
	}
}

func TestRegionFromBounds_multiple_resizes(t *testing.T) {
	rng := store.NewValueStore([2]float64{0, 100})

	RegionFromBounds(rng, gfx.RectFromXYWH(0, 0, 400, 300))
	if got := rng.Get(); got[1] != 400 {
		t.Fatalf("after first resize: hi = %f, want 400", got[1])
	}

	RegionFromBounds(rng, gfx.RectFromXYWH(0, 0, 800, 600))
	if got := rng.Get(); got[1] != 800 {
		t.Fatalf("after second resize: hi = %f, want 800", got[1])
	}
}

func TestRegionFromBounds_height_not_used(t *testing.T) {
	rng := store.NewValueStore([2]float64{0, 100})

	RegionFromBounds(rng, gfx.RectFromXYWH(50, 100, 300, 9999))

	got := rng.Get()
	if got[0] != 0 || got[1] != 300 {
		t.Fatalf("range = [%f,%f], want [0,300]", got[0], got[1])
	}
}

func TestRegionFromBounds_lo_always_zero(t *testing.T) {
	rng := store.NewValueStore([2]float64{5, 100})

	RegionFromBounds(rng, gfx.RectFromXYWH(-50, -50, 200, 150))

	got := rng.Get()
	if got[0] != 0 {
		t.Fatalf("lo must always be 0, got %f", got[0])
	}
	if got[1] <= 0 {
		t.Fatalf("expected positive hi, got %f", got[1])
	}
}
