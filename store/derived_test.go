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
