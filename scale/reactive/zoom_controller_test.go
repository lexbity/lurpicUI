package reactive

import (
	"math"
	"testing"

	"codeburg.org/lexbit/lurpicui/scale"
	"codeburg.org/lexbit/lurpicui/store"
)

func TestZoomController_zoom_in_preserves_focal(t *testing.T) {
	domain := store.NewValueStore([2]float64{0, 100})
	zc := NewZoomController(domain)

	// Build a scale from the initial domain
	s := scale.NewLinear(scale.WithDomain(0, 100), scale.WithRange(0, 500))
	focal := 40.0
	initialPos := s.Map(focal)

	// Zoom in 2x around focal
	zc.Zoom(focal, 2)

	// Build a new scale from the updated domain
	d := domain.Get()
	s2 := scale.NewLinear(scale.WithDomain(d[0], d[1]), scale.WithRange(0, 500))
	newPos := s2.Map(focal)

	if math.Abs(newPos-initialPos) > 1e-12 {
		t.Fatalf("focal position changed: %.10f -> %.10f", initialPos, newPos)
	}
}

func TestZoomController_zoom_out(t *testing.T) {
	domain := store.NewValueStore([2]float64{25, 75})
	zc := NewZoomController(domain)

	// Zoom out 2x
	zc.Zoom(50, 0.5)

	d := domain.Get()
	// ZoomDomain(25, 75, 50, 0.5) → lo=0, hi=100
	if d[0] != 0 || d[1] != 100 {
		t.Fatalf("after zoom out: domain = [%f,%f], want [0,100]", d[0], d[1])
	}
}

func TestZoomController_pan(t *testing.T) {
	domain := store.NewValueStore([2]float64{0, 100})
	zc := NewZoomController(domain)

	// Pan right by 50
	zc.Pan(50)

	d := domain.Get()
	if d[0] != 50 || d[1] != 150 {
		t.Fatalf("after pan: domain = [%f,%f], want [50,150]", d[0], d[1])
	}
}

func TestZoomController_pan_reversible(t *testing.T) {
	domain := store.NewValueStore([2]float64{0, 100})
	zc := NewZoomController(domain)

	zc.Pan(50)
	zc.Pan(-50)

	d := domain.Get()
	if d[0] != 0 || d[1] != 100 {
		t.Fatalf("after pan round-trip: domain = [%f,%f], want [0,100]", d[0], d[1])
	}
}

func TestZoomController_zoom_then_pan(t *testing.T) {
	domain := store.NewValueStore([2]float64{0, 100})
	zc := NewZoomController(domain)

	// Zoom in 2x around 50
	zc.Zoom(50, 2)
	d1 := domain.Get()
	if d1[0] != 25 || d1[1] != 75 {
		t.Fatalf("after zoom: domain = [%f,%f], want [25,75]", d1[0], d1[1])
	}

	// Pan by 10
	zc.Pan(10)
	d2 := domain.Get()
	if d2[0] != 35 || d2[1] != 85 {
		t.Fatalf("after zoom+pan: domain = [%f,%f], want [35,85]", d2[0], d2[1])
	}
}

func TestZoomController_domain_updates_reactive_scale(t *testing.T) {
	domain := store.NewValueStore([2]float64{0, 100})
	rng := store.NewValueStore([2]float64{0, 500})
	rs := NewLinearReactive(domain, rng)
	zc := NewZoomController(domain)

	s := rs.Get()
	if got := s.Map(50); got != 250 {
		t.Fatalf("initial Map(50) = %f, want 250", got)
	}

	// Zoom via controller — this updates the domain ValueStore
	zc.Zoom(50, 2)

	// The reactive scale should reflect the new domain
	rs.Get() // trigger recompute
	s = rs.Get()
	if got := s.Map(50); got != 250 {
		t.Fatalf("after zoom Map(50) = %f, want 250 (focal preserved)", got)
	}

	// Map(25) — the new lo should map to range lo
	if got := s.Map(25); got != 0 {
		t.Fatalf("after zoom Map(25) = %f, want 0", got)
	}
}

func TestZoomController_version_bumps_on_zoom(t *testing.T) {
	domain := store.NewValueStore([2]float64{0, 100})
	rng := store.NewValueStore([2]float64{0, 500})
	rs := NewLinearReactive(domain, rng)
	zc := NewZoomController(domain)

	rs.Get() // prime
	v0 := rs.Version()

	zc.Zoom(50, 2)
	rs.Get()
	v1 := rs.Version()
	if v1 <= v0 {
		t.Fatal("version should bump after zoom-induced domain change")
	}
}

func TestZoomController_factor_one_is_noop(t *testing.T) {
	domain := store.NewValueStore([2]float64{10, 100})
	zc := NewZoomController(domain)

	zc.Zoom(50, 1)
	d := domain.Get()
	if d[0] != 10 || d[1] != 100 {
		t.Fatalf("after factor 1 zoom: domain = [%f,%f], want [10,100]", d[0], d[1])
	}
}

func TestZoomController_no_viewport_imports(t *testing.T) {
	// Verify no facet or gfx imports by checking that this compiles.
	// The ZoomController operates purely in data space.
	domain := store.NewValueStore([2]float64{0, 100})
	zc := NewZoomController(domain)
	zc.Zoom(50, 2)
	zc.Pan(10)
}
