package store

import (
	"sync/atomic"
	"testing"

	"codeburg.org/lexbit/lurpicui/signal"
)

// TestDerivedPeek_NeverMutates is the G4.1 gate: Peek reports the last
// computed value and the dirty flag without recomputing, without bumping
// versions, without shifting cache keys, and without emitting OnChange.
func TestDerivedPeek_NeverMutates(t *testing.T) {
	src := NewValueStore(1)
	computations := atomic.Int64{}
	d := NewDerived(func() int {
		computations.Add(1)
		return src.Get() * 10
	}, src)

	// Prime once so a computed value exists.
	if got := d.Get(); got != 10 {
		t.Fatalf("Get = %d", got)
	}
	if comps := computations.Load(); comps != 1 {
		t.Fatalf("computations after prime = %d, want 1", comps)
	}
	versionBefore := d.Version()

	// Peek on a clean derived: clean value, no recompute, no version bump.
	value, dirty := d.Peek()
	if dirty {
		t.Fatal("clean derived must not report dirty")
	}
	if value != 10 {
		t.Fatalf("Peek value = %d, want 10", value)
	}
	if comps := computations.Load(); comps != 1 {
		t.Fatalf("Peek recomputed: %d computations, want 1", comps)
	}
	if d.Version() != versionBefore {
		t.Fatal("Peek must not bump the derived version")
	}

	// Dirty the source: Peek reports dirty with the STALE value, still no
	// recompute.
	src.Set(2)
	value, dirty = d.Peek()
	if !dirty {
		t.Fatal("stale derived must report dirty")
	}
	if value != 10 {
		t.Fatalf("Peek must return the last computed value, got %d", value)
	}
	if comps := computations.Load(); comps != 1 {
		t.Fatalf("Peek recomputed a dirty derived: %d, want 1", comps)
	}
	if d.Version() != versionBefore {
		t.Fatal("dirty Peek must not bump the derived version")
	}

	// Cache keys unshifted: a subsequent Get recomputes exactly once and
	// sees the new value.
	if got := d.Get(); got != 20 {
		t.Fatalf("Get after dirty = %d, want 20", got)
	}
	if comps := computations.Load(); comps != 2 {
		t.Fatalf("computations after Get = %d, want 2", comps)
	}

	// No OnChange fired by any Peek: subscribe, Peek, require silence.
	changes := 0
	id := d.OnChange.Subscribe(func(signal.Change[int]) { changes++ })
	defer d.OnChange.Unsubscribe(id)
	_, _ = d.Peek()
	src.Set(3)
	_, _ = d.Peek()
	if changes != 0 {
		t.Fatalf("Peek emitted OnChange %d times, want 0", changes)
	}
}

// TestDerivedPeek_FreshDerived: a never-computed derived peeks as the zero
// value, dirty.
func TestDerivedPeek_FreshDerived(t *testing.T) {
	src := NewValueStore("x")
	d := NewDerived(func() string { return src.Get() + "!" }, src)
	value, dirty := d.Peek()
	if !dirty {
		t.Fatal("fresh derived must report dirty")
	}
	if value != "" {
		t.Fatalf("fresh Peek value = %q, want the zero value", value)
	}
}
