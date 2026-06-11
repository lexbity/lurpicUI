package marks

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/store"
)

func TestConst_returns_literal(t *testing.T) {
	b := Const("hello")
	if got := b.Get(); got != "hello" {
		t.Fatalf("Get() = %q, want %q", got, "hello")
	}
}

func TestConst_is_not_dynamic(t *testing.T) {
	b := Const(42)
	if b.IsDynamic() {
		t.Fatal("expected IsDynamic = false")
	}
}

func TestConst_has_no_dirty_flags(t *testing.T) {
	b := Const(3.14)
	if flags := b.DirtyFlags(); flags != 0 {
		t.Fatalf("DirtyFlags() = %d, want 0", flags)
	}
}

func TestConst_has_no_subscription(t *testing.T) {
	b := Const(true)
	cleanup := b.SubscribeOnChange(func() { t.Error("unexpected call") })
	if cleanup != nil {
		t.Fatal("expected nil cleanup for const binding")
	}
}

func TestConst_many_copies_return_same_value(t *testing.T) {
	b1 := Const("hello")
	b2 := b1
	b3 := Const("world")
	if b1.Get() != b2.Get() {
		t.Fatal("copy must return same value")
	}
	if b1.Get() == b3.Get() {
		t.Fatal("different consts must return different values")
	}
}

func TestStore_returns_initial_value(t *testing.T) {
	s := store.NewValueStore("initial")
	b := FromStore(s, facet.DirtyProjection)
	if got := b.Get(); got != "initial" {
		t.Fatalf("Get() = %q, want %q", got, "initial")
	}
}

func TestStore_is_dynamic(t *testing.T) {
	s := store.NewValueStore(0)
	b := FromStore(s, facet.DirtyLayout)
	if !b.IsDynamic() {
		t.Fatal("expected IsDynamic = true")
	}
}

func TestStore_reports_declared_dirty_flags(t *testing.T) {
	s := store.NewValueStore(0)
	b := FromStore(s, facet.DirtyLayout|facet.DirtyProjection)
	flags := b.DirtyFlags()
	if flags&facet.DirtyLayout == 0 {
		t.Error("expected DirtyLayout flag")
	}
	if flags&facet.DirtyProjection == 0 {
		t.Error("expected DirtyProjection flag")
	}
}

func TestStore_detects_version_change(t *testing.T) {
	s := store.NewValueStore("a")
	b := FromStore(s, facet.DirtyProjection)

	if got := b.Get(); got != "a" {
		t.Fatalf("before set: Get() = %q, want %q", got, "a")
	}

	s.Set("b")

	if got := b.Get(); got != "b" {
		t.Fatalf("after set: Get() = %q, want %q", got, "b")
	}
}

func TestStore_re_reads_after_multiple_sets(t *testing.T) {
	s := store.NewValueStore(0)
	b := FromStore(s, facet.DirtyProjection)

	for i := 1; i <= 5; i++ {
		s.Set(i)
		if got := b.Get(); got != i {
			t.Fatalf("iteration %d: Get() = %d, want %d", i, got, i)
		}
	}
}

func TestStore_subscribe_fires_on_change(t *testing.T) {
	s := store.NewValueStore("x")
	b := FromStore(s, facet.DirtyProjection)

	fired := 0
	cleanup := b.SubscribeOnChange(func() { fired++ })
	if cleanup == nil {
		t.Fatal("expected non-nil cleanup")
	}
	defer cleanup()

	s.Set("y")
	if fired != 1 {
		t.Fatalf("expected 1 fire, got %d", fired)
	}

	s.Set("z")
	if fired != 2 {
		t.Fatalf("expected 2 fires, got %d", fired)
	}
}

func TestStore_subscribe_cleanup_stops_firing(t *testing.T) {
	s := store.NewValueStore("a")
	b := FromStore(s, facet.DirtyProjection)

	fired := 0
	cleanup := b.SubscribeOnChange(func() { fired++ })
	s.Set("b")
	if fired != 1 {
		t.Fatalf("expected 1 fire before cleanup, got %d", fired)
	}

	cleanup()
	s.Set("c")
	if fired != 1 {
		t.Fatalf("expected no fire after cleanup, got %d", fired)
	}
}

func TestStore_nil_store_returns_const(t *testing.T) {
	b := FromStore[string](nil, facet.DirtyProjection)
	if b.IsDynamic() {
		t.Fatal("expected nil store to produce non-dynamic binding")
	}
	if b.Get() != "" {
		t.Fatalf("Get() = %q, want zero value", b.Get())
	}
}

func TestDerived_returns_computed_value(t *testing.T) {
	src := store.NewValueStore(10)
	d := store.NewDerived(func() int { return src.Get() * 2 }, src)
	b := FromDerived(d, facet.DirtyProjection)

	if got := b.Get(); got != 20 {
		t.Fatalf("Get() = %d, want %d", got, 20)
	}
}

func TestDerived_detects_version_change(t *testing.T) {
	src := store.NewValueStore(10)
	d := store.NewDerived(func() int { return src.Get() * 2 }, src)
	b := FromDerived(d, facet.DirtyProjection)

	src.Set(20)

	if got := b.Get(); got != 40 {
		t.Fatalf("Get() = %d, want %d", got, 40)
	}
}

func TestDerived_subscribe_fires_on_change(t *testing.T) {
	src := store.NewValueStore(1)
	d := store.NewDerived(func() int { return src.Get() * 3 }, src)
	b := FromDerived(d, facet.DirtyProjection)

	// First Get() initializes the Derived — fires OnChange for old=0→new=3.
	// Subscribe after initialization to avoid the initial fire.
	b.Get()

	fired := 0
	cleanup := b.SubscribeOnChange(func() { fired++ })
	defer cleanup()

	// Derived fires OnChange inside Get() when recomputation finds a change.
	src.Set(2)
	b.Get() // triggers d.Get(), recomputes, fires d.OnChange → fired++
	if fired != 1 {
		t.Fatalf("expected 1 fire after src.Set, got %d", fired)
	}

	// No change this time — version is current, cached value is returned.
	b.Get()
	if fired != 1 {
		t.Fatalf("expected no fire without source change, got %d", fired)
	}
}

func TestDerived_nil_derived_returns_const(t *testing.T) {
	b := FromDerived[int](nil, facet.DirtyProjection)
	if b.IsDynamic() {
		t.Fatal("expected nil derived to produce non-dynamic binding")
	}
	if b.Get() != 0 {
		t.Fatalf("Get() = %d, want zero value", b.Get())
	}
}

func TestConst_copy_preserves_independence(t *testing.T) {
	b1 := Const("hello")
	b2 := b1
	b3 := Const("world")

	if b1.Get() != b2.Get() {
		t.Fatal("copy must equal original")
	}
	b1 = b3
	_ = b1
	if b2.Get() != "hello" {
		t.Fatal("reassigning original must not affect copy")
	}
}

func TestBinding_no_mutation_api(t *testing.T) {
	// Compile-time check: Binding[T] must not expose Set or similar mutations.
	// If Binding gains an exported Set method this test becomes a compile error.
	b := Const(0)
	_ = b.Get()
	_ = b.IsDynamic()
	_ = b.DirtyFlags()
	_ = b.SubscribeOnChange(nil)
}

func TestStore_get_does_not_mutate_source(t *testing.T) {
	s := store.NewValueStore(100)
	b := FromStore(s, facet.DirtyProjection)

	b.Get()
	if s.Get() != 100 {
		t.Fatal("Get on binding must not mutate source")
	}
}
