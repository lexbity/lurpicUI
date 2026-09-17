package store

import (
	"sync/atomic"
	"testing"

	"codeburg.org/lexbit/lurpicui/internal/syncutil"
	"codeburg.org/lexbit/lurpicui/signal"
)

func TestDerived_computes_initial_value(t *testing.T) {
	a := NewValueStore(2)
	b := NewValueStore(3)
	d := NewDerived(func() int { return a.Get() + b.Get() }, a, b)

	if got := d.Get(); got != 5 {
		t.Fatalf("got %d", got)
	}
	if got := d.Version(); got == 0 {
		t.Fatal("expected derived version to increment")
	}
}

func TestDerived_recomputes_when_source_changes(t *testing.T) {
	a := NewValueStore(2)
	d := NewDerived(func() int { return a.Get() * 2 }, a)
	if got := d.Get(); got != 4 {
		t.Fatalf("got %d", got)
	}
	a.Set(5)
	if got := d.Get(); got != 10 {
		t.Fatalf("got %d", got)
	}
}

func TestDerived_lazy_does_not_recompute_until_get(t *testing.T) {
	a := NewValueStore(1)
	calls := 0
	d := NewDerived(func() int {
		calls++
		return a.Get()
	}, a)

	_ = d.Get()
	a.Set(2)
	if calls != 1 {
		t.Fatalf("calls = %d", calls)
	}
	_ = d.Get()
	if calls != 2 {
		t.Fatalf("calls = %d", calls)
	}
}

func TestDerived_recomputation_always_emits(t *testing.T) {
	a := NewValueStore(1)
	d := NewDerived(func() int { return a.Get() % 2 }, a)
	_ = d.Get()
	var called int32
	d.OnChange.Subscribe(func(signal.Change[int]) { called++ })
	a.Set(3)
	_ = d.Get()

	if called == 0 {
		t.Fatal("expected signal from recomputed derived value")
	}
}

func TestDerived_multiple_sources(t *testing.T) {
	a := NewValueStore(1)
	b := NewValueStore(2)
	d := NewDerived(func() int { return a.Get() + b.Get() }, a, b)
	if got := d.Get(); got != 3 {
		t.Fatalf("got %d", got)
	}
	a.Set(5)
	if got := d.Get(); got != 7 {
		t.Fatalf("got %d", got)
	}
	b.Set(6)
	if got := d.Get(); got != 11 {
		t.Fatalf("got %d", got)
	}
}

func TestDerived_version_increments_on_change(t *testing.T) {
	a := NewValueStore(1)
	d := NewDerived(func() int { return a.Get() }, a)
	_ = d.Get()
	ver := d.Version()
	a.Set(2)
	_ = d.Get()
	if got := d.Version(); got <= ver {
		t.Fatalf("version = %d -> %d", ver, got)
	}
}

func TestDerived_of_derived_propagates(t *testing.T) {
	syncutil.ResetRuntimeThreadForTest()
	t.Cleanup(syncutil.ResetRuntimeThreadForTest)
	syncutil.RegisterRuntimeThread()

	a := NewValueStore(10)
	b := NewDerived(func() int { return a.Get() * 2 }, a)
	c := NewDerived(func() int { return b.Get() + 1 }, b)

	if got := c.Get(); got != 21 {
		t.Fatalf("initial c = %d, want 21", got)
	}

	a.Set(20)

	// b is now dirty but not recomputed (lazy). Reading b triggers
	// recompute, which increments b.Version and fires c's markDirty.
	bv := b.Version()
	b.Get()
	if b.Version() == bv {
		t.Fatal("b version did not change after Get following source change")
	}

	// c is now dirty. After c.Get(), the chain should produce the
	// correct value flowing through the intermediate Derived.
	if got := c.Get(); got != 41 {
		t.Fatalf("after a=20, c = %d, want 41", got)
	}
}

func TestDerived_diamond_recomputes_once(t *testing.T) {
	syncutil.ResetRuntimeThreadForTest()
	t.Cleanup(syncutil.ResetRuntimeThreadForTest)
	syncutil.RegisterRuntimeThread()

	a := NewValueStore(1)

	var bCalls, cCalls, dCalls int32
	b := NewDerived(func() int { atomic.AddInt32(&bCalls, 1); return a.Get() + 10 }, a)
	c := NewDerived(func() int { atomic.AddInt32(&cCalls, 1); return a.Get() + 100 }, a)
	d := NewDerived(func() int {
		atomic.AddInt32(&dCalls, 1)
		return b.Get() + c.Get()
	}, b, c)

	b.Get()
	c.Get()
	if got := d.Get(); got != 112 {
		t.Fatalf("initial d = %d, want 112 (1+10 + 1+100)", got)
	}

	atomic.StoreInt32(&bCalls, 0)
	atomic.StoreInt32(&cCalls, 0)
	atomic.StoreInt32(&dCalls, 0)

	a.Set(5)

	// Read the intermediate Deriveds to propagate the invalidation.
	// b and c each recompute and fire d.markDirty twice, but d's
	// compute function should run exactly once when we read d.
	b.Get()
	c.Get()

	if got := d.Get(); got != 120 {
		t.Fatalf("after a=5, d = %d, want 120 (5+10 + 5+100)", got)
	}
	if n := atomic.LoadInt32(&dCalls); n != 1 {
		t.Fatalf("d compute ran %d times, want 1", n)
	}
}

func TestDerived_no_sources_is_constant(t *testing.T) {
	syncutil.ResetRuntimeThreadForTest()
	t.Cleanup(syncutil.ResetRuntimeThreadForTest)
	syncutil.RegisterRuntimeThread()

	var calls int32
	d := NewDerived(func() int {
		atomic.AddInt32(&calls, 1)
		return 42
	})

	if got := d.Get(); got != 42 {
		t.Fatalf("got %d, want 42", got)
	}
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Fatalf("compute calls = %d, want 1", n)
	}

	// Second read should use cached value.
	if got := d.Get(); got != 42 {
		t.Fatalf("second read = %d, want 42", got)
	}
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Fatalf("compute calls = %d after second read, want 1", n)
	}

	// Version must be non-zero after initial computation.
	if d.Version() == 0 {
		t.Fatal("expected non-zero version after compute")
	}
}

func TestDerived_nil_compute_returns_zero(t *testing.T) {
	d := NewDerived[int](nil)
	if got := d.Get(); got != 0 {
		t.Fatalf("got %d, want 0", got)
	}
}

func TestDerived_non_versioned_source_is_detected(t *testing.T) {
	type nonVersionedSrc struct {
		Invalidatable
	}
	src := nonVersionedSrc{}
	panicked := false
	func() {
		defer func() { panicked = recover() != nil }()
		NewDerived(func() int { return 1 }, src)
	}()
	if !panicked {
		t.Fatal("expected panic for non-versioned Derived source, got none")
	}
}

func TestDerived_onInvalidated_fires_on_clean_to_dirty_transition(t *testing.T) {
	a := NewValueStore(1)
	d := NewDerived(func() int { return a.Get() }, a)
	_ = d.Get() // initialize and settle clean

	var fired int32
	id := d.OnInvalidated.Subscribe(func(struct{}) { fired++ })
	defer d.OnInvalidated.Unsubscribe(id)

	a.Set(2)
	if fired != 1 {
		t.Fatalf("OnInvalidated fired %d times after one clean->dirty transition, want 1", fired)
	}

	// A read settles clean again; the next write fires once more.
	_ = d.Get()
	a.Set(3)
	if fired != 2 {
		t.Fatalf("OnInvalidated fired %d times after second transition, want 2", fired)
	}
}

func TestDerived_onInvalidated_coalesces_multiple_writes(t *testing.T) {
	var queue []func()
	SetSignalQueueHook(func(fn func()) { queue = append(queue, fn) })
	defer SetSignalQueueHook(nil)

	a := NewValueStore(1)
	d := NewDerived(func() int { return a.Get() }, a)
	_ = d.Get() // initialize and settle clean

	var fired int32
	id := d.OnInvalidated.Subscribe(func(struct{}) { fired++ })
	defer d.OnInvalidated.Unsubscribe(id)

	// N upstream writes inside one dirty period produce exactly one
	// notification (RX-1 Q2 transition-coalescing).
	a.Set(2)
	a.Set(3)
	a.Set(4)

	// Drain the notification queue, then the coalesced OnInvalidated emission.
	for len(queue) > 0 {
		batch := append([]func(){}, queue...)
		queue = queue[:0]
		for _, fn := range batch {
			fn()
		}
	}
	if fired != 1 {
		t.Fatalf("OnInvalidated fired %d times for 3 writes in one dirty period, want 1", fired)
	}

	// The recompute reflects the latest write.
	if got := d.Get(); got != 4 {
		t.Fatalf("derived = %d, want 4", got)
	}
}

func TestDerived_onInvalidated_no_emission_before_initialized(t *testing.T) {
	a := NewValueStore(1)
	d := NewDerived(func() int { return a.Get() }, a)
	// Never Get()'d: the derived is dirty from construction and is never
	// "clean", so a write before the first read must not emit (RX-1 FR-2
	// corollary: the first projection is uncached and therefore fresh).

	var fired int32
	id := d.OnInvalidated.Subscribe(func(struct{}) { fired++ })
	defer d.OnInvalidated.Unsubscribe(id)

	a.Set(2)
	if fired != 0 {
		t.Fatalf("OnInvalidated fired %d times before first Get, want 0", fired)
	}

	// After initialization settles clean, a write emits.
	_ = d.Get()
	a.Set(3)
	if fired != 1 {
		t.Fatalf("OnInvalidated fired %d times after init+write, want 1", fired)
	}
}

func TestDerived_onInvalidated_precedes_recomputed_onchange(t *testing.T) {
	a := NewValueStore(1)
	d := NewDerived(func() int { return a.Get() * 10 }, a)
	_ = d.Get() // initialize: value 10

	var invalidated int
	var changed int
	var valueAtInvalidation int
	idI := d.OnInvalidated.Subscribe(func(struct{}) {
		invalidated++
		// Mimic the FromDerived binding (RX-1 FR-2): force the recompute in
		// the signal-delivery phase so the fresh value is ready for projection.
		valueAtInvalidation = d.Get()
	})
	idC := d.OnChange.Subscribe(func(c signal.Change[int]) { changed++ })
	defer d.OnInvalidated.Unsubscribe(idI)
	defer d.OnChange.Unsubscribe(idC)

	a.Set(2)

	// OnInvalidated fires eagerly on the transition; the binding's forced
	// Get() recomputes within that delivery, so OnChange fires once as a
	// consequence and the recomputed value is already fresh.
	if invalidated != 1 {
		t.Fatalf("OnInvalidated fired %d times, want 1", invalidated)
	}
	if valueAtInvalidation != 20 {
		t.Fatalf("value read during OnInvalidated = %d, want 20 (recomputed in signal phase)", valueAtInvalidation)
	}
	if changed != 1 {
		t.Fatalf("OnChange fired %d times, want 1 (from the forced recompute)", changed)
	}
}

func TestDerived_onInvalidated_race_concurrent_set_get(t *testing.T) {
	syncutil.ResetRuntimeThreadForTest()
	t.Cleanup(syncutil.ResetRuntimeThreadForTest)
	syncutil.RegisterRuntimeThread()

	a := NewValueStore(0)
	d := NewDerived(func() int { return a.Get() }, a)

	var fired int32
	id := d.OnInvalidated.Subscribe(func(struct{}) { fired++ })
	defer d.OnInvalidated.Unsubscribe(id)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 500; i++ {
			_ = d.Get()
		}
	}()
	for i := 1; i <= 500; i++ {
		a.Set(i)
	}
	<-done
	_ = fired
}
