package store

import (
	"testing"

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
