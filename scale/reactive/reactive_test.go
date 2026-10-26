package reactive

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/scale"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
)

func TestNewLinearReactive_basic(t *testing.T) {
	scope := store.NewScope()
	defer scope.Close()
	domain := store.NewValueStore(scope, [2]float64{0, 100})
	rng := store.NewValueStore(scope, [2]float64{0, 500})
	rs := NewLinearReactive(scope, domain, rng)

	s := rs.Get()
	if s.Kind() != scale.KindLinear {
		t.Fatalf("kind = %s, want KindLinear", s.Kind())
	}
	if got := s.Map(50); got != 250 {
		t.Fatalf("Map(50) = %f, want 250", got)
	}
}

// TestBridgeDerived_eagerUpdateWithoutExternalGet pins RX-1 FR-2: the Derived→
// ValueStore bridge subscribes OnInvalidated and recomputes on the clean→dirty
// transition itself, so a scale built from derived sources stays live with zero
// external Get() flushing (the chart's old arrange() force-flush pattern).
func TestBridgeDerived_eagerUpdateWithoutExternalGet(t *testing.T) {
	scope := store.NewScope()
	defer scope.Close()
	dsrc := store.NewValueStore(scope, [2]float64{0, 10})
	dd := store.NewDerived(scope, func() [2]float64 { return dsrc.Get() }, dsrc)
	rsrc := store.NewValueStore(scope, [2]float64{0, 100})
	rd := store.NewDerived(scope, func() [2]float64 { return rsrc.Get() }, rsrc)
	rs := NewLinearReactiveFromDerived(scope, dd, rd)

	before := rs.Get()
	if got := before.Map(5); got != 50 {
		t.Fatalf("initial Map(5) = %f, want 50", got)
	}

	// Change the source. The bridge must update dd's mirrored ValueStore via
	// OnInvalidated (no external dd.Get() here); the scale's derived then sees
	// the source version change and recomputes on its own Get.
	dsrc.Set([2]float64{0, 20})
	scope.FlushSignals()
	after := rs.Get()
	if got := after.Map(5); got != 25 {
		t.Fatalf("post-change Map(5) = %f, want 25 (domain now 0..20 over range 0..100)", got)
	}
}

func TestNewLogReactive_basic(t *testing.T) {
	scope := store.NewScope()
	defer scope.Close()
	domain := store.NewValueStore(scope, [2]float64{1, 1000})
	rng := store.NewValueStore(scope, [2]float64{0, 500})
	rs := NewLogReactive(scope, domain, rng)

	s := rs.Get()
	if s.Kind() != scale.KindLog {
		t.Fatalf("kind = %s, want KindLog", s.Kind())
	}
}

func TestNewTimeReactive_basic(t *testing.T) {
	scope := store.NewScope()
	defer scope.Close()
	domain := store.NewValueStore(scope, [2]float64{0, 1000})
	rng := store.NewValueStore(scope, [2]float64{0, 500})
	rs := NewTimeReactive(scope, domain, rng)

	s := rs.Get()
	if s.Kind() != scale.KindTime {
		t.Fatalf("kind = %s, want KindTime", s.Kind())
	}
}

func TestReactiveScale_version_bumps_on_domain_change(t *testing.T) {
	scope := store.NewScope()
	defer scope.Close()
	domain := store.NewValueStore(scope, [2]float64{0, 100})
	rng := store.NewValueStore(scope, [2]float64{0, 500})
	rs := NewLinearReactive(scope, domain, rng)

	rs.Get() // prime cache (version 1)
	v0 := rs.Version()
	domain.Set([2]float64{0, 200})
	rs.Get() // recompute → version bumps
	v1 := rs.Version()
	if v1 <= v0 {
		t.Fatalf("version should bump after domain change: %d -> %d", v0, v1)
	}
}

func TestReactiveScale_scale_updates_after_domain_change(t *testing.T) {
	scope := store.NewScope()
	defer scope.Close()
	domain := store.NewValueStore(scope, [2]float64{0, 100})
	rng := store.NewValueStore(scope, [2]float64{0, 500})
	rs := NewLinearReactive(scope, domain, rng)

	// Map before change
	s1 := rs.Get()
	if got := s1.Map(50); got != 250 {
		t.Fatalf("before: Map(50) = %f, want 250", got)
	}

	// Change domain
	domain.Set([2]float64{0, 200})

	// Map after change
	s2 := rs.Get()
	if got := s2.Map(50); got != 125 {
		t.Fatalf("after: Map(50) = %f, want 125", got)
	}
}

func TestReactiveScale_no_recompute_when_unchanged(t *testing.T) {
	scope := store.NewScope()
	defer scope.Close()
	domain := store.NewValueStore(scope, [2]float64{0, 100})
	rng := store.NewValueStore(scope, [2]float64{0, 500})
	rs := NewLinearReactive(scope, domain, rng)

	rs.Get() // prime cache → version 1
	v0 := rs.Version()

	// No changes — repeated Get should not bump version
	rs.Get()
	v1 := rs.Version()
	if v1 != v0 {
		t.Fatalf("version should not bump without source changes: %d -> %d", v0, v1)
	}
}

func TestReactiveScale_version_bumps_on_range_change(t *testing.T) {
	scope := store.NewScope()
	defer scope.Close()
	domain := store.NewValueStore(scope, [2]float64{0, 100})
	rng := store.NewValueStore(scope, [2]float64{0, 500})
	rs := NewLinearReactive(scope, domain, rng)

	rs.Get() // prime cache
	v0 := rs.Version()
	rng.Set([2]float64{0, 1000})
	rs.Get() // recompute
	v1 := rs.Version()
	if v1 <= v0 {
		t.Fatalf("version should bump after range change: %d -> %d", v0, v1)
	}
}

func TestReactiveScale_get_after_stale(t *testing.T) {
	scope := store.NewScope()
	defer scope.Close()
	domain := store.NewValueStore(scope, [2]float64{0, 100})
	rng := store.NewValueStore(scope, [2]float64{0, 500})
	rs := NewLinearReactive(scope, domain, rng)

	// Initial Get
	s := rs.Get()
	if got := s.Map(0); got != 0 {
		t.Fatalf("Map(0) = %f, want 0", got)
	}

	// Change range
	rng.Set([2]float64{50, 100})

	// Get should return updated scale
	s = rs.Get()
	if got := s.Map(0); got != 50 {
		t.Fatalf("after range change: Map(0) = %f, want 50", got)
	}
}

func TestReactiveScale_supports_scale_interface(t *testing.T) {
	scope := store.NewScope()
	defer scope.Close()
	domain := store.NewValueStore(scope, [2]float64{0, 100})
	rng := store.NewValueStore(scope, [2]float64{0, 500})
	rs := NewLinearReactive(scope, domain, rng)

	var sc scale.Scale = rs.Get()
	_ = sc
}

func TestReactiveScale_supports_invertible_interface(t *testing.T) {
	scope := store.NewScope()
	defer scope.Close()
	domain := store.NewValueStore(scope, [2]float64{0, 100})
	rng := store.NewValueStore(scope, [2]float64{0, 500})
	rs := NewLinearReactive(scope, domain, rng)

	var inv = rs.Get()
	_ = inv
}

func TestReactiveScale_multiple_independent_scales(t *testing.T) {
	scope :=
		// Two reactive scales sharing the same domain store
		store.NewScope()
	defer scope.Close()

	domain := store.NewValueStore(scope, [2]float64{0, 100})
	rng1 := store.NewValueStore(scope, [2]float64{0, 500})
	rng2 := store.NewValueStore(scope, [2]float64{0, 1000})

	rs1 := NewLinearReactive(scope, domain, rng1)
	rs2 := NewLinearReactive(scope, domain, rng2)

	// Both reflect the same domain
	s1 := rs1.Get()
	s2 := rs2.Get()
	if got := s1.Map(50); got != 250 {
		t.Fatalf("rs1 Map(50) = %f, want 250", got)
	}
	if got := s2.Map(50); got != 500 {
		t.Fatalf("rs2 Map(50) = %f, want 500", got)
	}

	// Change domain — both should update
	domain.Set([2]float64{0, 200})
	if got := rs1.Get().Map(50); got != 125 {
		t.Fatalf("rs1 after: Map(50) = %f, want 125", got)
	}
	if got := rs2.Get().Map(50); got != 250 {
		t.Fatalf("rs2 after: Map(50) = %f, want 250", got)
	}
}

func TestReactiveScale_with_options(t *testing.T) {
	scope := store.NewScope()
	defer scope.Close()
	domain := store.NewValueStore(scope, [2]float64{0, 100})
	rng := store.NewValueStore(scope, [2]float64{0, 500})
	rs := NewLinearReactive(scope, domain, rng, scale.WithClamp(scale.OutOfRangeClamp))

	// Clamp should be active
	s := rs.Get()
	if got := s.Map(200); got != 500 {
		t.Fatalf("Map(200) with clamp = %f, want 500", got)
	}

	// Change domain, clamp should still apply
	domain.Set([2]float64{0, 200})
	s = rs.Get()
	if got := s.Map(300); got != 500 {
		t.Fatalf("after domain change, Map(300) with clamp = %f, want 500", got)
	}
}

func TestNewLogReactive_panics_on_invalid_domain(t *testing.T) {
	scope := store.NewScope()
	defer scope.Close()
	domain := store.NewValueStore(scope, [2]float64{-1, 10}) // crosses zero
	rng := store.NewValueStore(scope, [2]float64{0, 500})
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for invalid log domain")
		}
	}()
	rs := NewLogReactive(scope, domain, rng)
	rs.Get() // triggers the compute function which panics
}

func TestReactiveScale_log_with_options(t *testing.T) {
	scope := store.NewScope()
	defer scope.Close()
	domain := store.NewValueStore(scope, [2]float64{1, 1000})
	rng := store.NewValueStore(scope, [2]float64{0, 500})
	rs := NewLogReactive(scope, domain, rng, scale.WithBase(2))

	s := rs.Get()
	if s.Kind() != scale.KindLog {
		t.Fatalf("kind = %s, want KindLog", s.Kind())
	}
}

func TestReactiveScale_onchange_fires(t *testing.T) {
	scope :=
		// ReactiveScale's underlying store.Derived fires OnChange on recompute.
		// We verify this by subscribing to the derived's OnChange directly.
		store.NewScope()
	defer scope.Close()

	domain := store.NewValueStore(scope, [2]float64{0, 100})
	rng := store.NewValueStore(scope, [2]float64{0, 500})

	fired := 0
	derived := store.NewDerived(scope, func() scale.InvertibleScale {
		d := domain.Get()
		r := rng.Get()
		return scale.NewLinear(
			scale.WithDomain(d[0], d[1]),
			scale.WithRange(r[0], r[1]),
		)
	},
		domain, rng,
	)
	derived.OnChange.Subscribe(func(c signal.Change[scale.InvertibleScale]) {
		fired++
	})

	derived.Get() // initial compute (fires OnChange)
	scope.FlushSignals()
	if fired == 0 {
		t.Fatal("expected OnChange to fire on initial compute")
	}

	// Change domain — triggers recompute on next Get
	domain.Set([2]float64{0, 200})
	scope.FlushSignals()
	before := fired
	derived.Get()
	scope.FlushSignals()
	if fired <= before {
		t.Fatal("expected OnChange to fire after domain change")
	}
}

// --- benchmarks ---

func BenchmarkReactiveScaleGet(b *testing.B) {
	scope := store.NewScope()
	defer scope.Close()
	domain := store.NewValueStore(scope, [2]float64{0, 100})
	rng := store.NewValueStore(scope, [2]float64{0, 500})
	rs := NewLinearReactive(scope, domain, rng)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rs.Get()
	}
}
