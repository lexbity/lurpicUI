package marks

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
)

// --- Fixture ---

// valueTestMark is a minimal value-holding mark: a mirror field kept fresh by
// the binding's apply callback, the (a)–(d) contract observable directly.
type valueTestMark struct {
	Core
	mirror string
	bind   *ScalarBinding[string]
}

func newValueTestMark(external *store.ValueStore[string]) *valueTestMark {
	m := &valueTestMark{}
	m.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: 100, H: 50}}
	}
	m.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		m.Layout.ArrangedBounds = bounds
	}
	m.RegisterRoles(m)
	m.bind = BindScalar(m, external, facet.DirtyProjection, func(v string) {
		m.mirror = v
	})
	return m
}

func (m *valueTestMark) Base() *facet.Facet {
	m.BindImpl(m)
	return &m.Facet
}

func (m *valueTestMark) OnAttach(ctx facet.AttachContext) {
	m.Core.OnAttach(ctx)
	m.bind.Attach(m)
}

func (m *valueTestMark) OnDetach() { m.Core.OnDetach() }
func (m *valueTestMark) OnActivate() {
	m.Core.OnActivate()
}
func (m *valueTestMark) OnDeactivate() { m.Core.OnDeactivate() }

// edit applies a user edit through the contract write-back (c).
func (m *valueTestMark) edit(v string) {
	m.mirror = v
	m.bind.Write(v)
}

// --- Contract tests (RX-2 P4) ---

// (a) attach-time sync: the mark adopts the store's current value.
func TestScalarBinding_attachSync_fromPresetStore(t *testing.T) {
	s := store.NewValueStore("preset")
	m := newValueTestMark(s)
	facet.Attach(m, facet.AttachContext{})
	if m.mirror != "preset" {
		t.Fatalf("attach-time sync: mirror = %q, want %q", m.mirror, "preset")
	}
}

// (b) re-sync: an external store write mid-session re-applies within the
// delivery and invalidates the mark.
func TestScalarBinding_resync_externalWrite(t *testing.T) {
	s := store.NewValueStore("a")
	m := newValueTestMark(s)
	facet.Attach(m, facet.AttachContext{})
	s.Set("b")
	if m.mirror != "b" {
		t.Fatalf("re-sync: mirror = %q, want %q", m.mirror, "b")
	}
	if m.Base().DirtyFlags()&facet.DirtyProjection == 0 {
		t.Fatal("re-sync did not invalidate the mark's projection")
	}
}

// (c)+(d) write-back + echo guard: N consecutive user edits produce exactly
// N store changes, and no change notification re-enters the mark's apply.
func TestScalarBinding_writeBack_echoGuard(t *testing.T) {
	s := store.NewValueStore("")
	changes := 0
	s.OnChange.Subscribe(func(signal.Change[string]) { changes++ })
	m := newValueTestMark(s)
	facet.Attach(m, facet.AttachContext{})

	applies := 0
	m.bind.apply = func(v string) { applies++; m.mirror = v }

	for i, v := range []string{"one", "two", "three", "four"} {
		m.edit(v)
		if changes != i+1 {
			t.Fatalf("edit %d: store changes = %d, want %d", i+1, changes, i+1)
		}
		if m.mirror != v {
			t.Fatalf("edit %d: mirror = %q, want %q", i+1, m.mirror, v)
		}
	}
	if applies != 0 {
		t.Fatalf("echo guard failed: the mark's own writes re-entered apply %d times", applies)
	}
}

// (d) documented consequence: a store change to the value the mark already
// holds does not re-apply — the value is already applied.
func TestScalarBinding_echoGuard_externalSameValueSwallowed(t *testing.T) {
	s := store.NewValueStore("")
	m := newValueTestMark(s)
	facet.Attach(m, facet.AttachContext{})

	applies := 0
	m.bind.apply = func(v string) { applies++; m.mirror = v }

	m.edit("v1")
	s.Set("v1") // no store change → no emission; the mark already holds it
	if applies != 0 {
		t.Fatalf("external write of the held value re-applied (%d times)", applies)
	}
	// A different external value still re-syncs.
	s.Set("v2")
	if applies != 1 || m.mirror != "v2" {
		t.Fatalf("external differing write: applies = %d, mirror = %q", applies, m.mirror)
	}
}

// (d) the echo guard must not diverge mark and store: a user write, then an
// external write, then an external write back to the user's value — the last
// write is a real change and MUST re-sync (the naive lastWritten guard
// swallows it and the mark serves stale state).
func TestScalarBinding_echoGuard_externalInterleaveResyncs(t *testing.T) {
	s := store.NewValueStore("")
	m := newValueTestMark(s)
	facet.Attach(m, facet.AttachContext{})

	m.edit("v1")
	s.Set("v2")
	s.Set("v1")
	if m.mirror != "v1" {
		t.Fatalf("external write back to the user's value was swallowed: mirror = %q, store = %q", m.mirror, s.Get())
	}
}

// Dispose ends delivery: a store write after dispose must not touch the mark
// and must not panic (the subscription bag is released with the facet).
func TestScalarBinding_dispose_stopsDelivery(t *testing.T) {
	s := store.NewValueStore("a")
	m := newValueTestMark(s)
	facet.Attach(m, facet.AttachContext{})
	facet.Dispose(m)
	s.Set("b") // must not panic or deliver
	if m.mirror != "a" {
		t.Fatalf("write after dispose reached the disposed mark: %q", m.mirror)
	}
}

// A panicking apply is quarantined through the recovery hook, not propagated.
func TestScalarBinding_applyPanic_quarantined(t *testing.T) {
	facet.SetCallbackRecoveryHook(func(id facet.FacetID, role string, cb func()) (ran bool) {
		defer func() {
			if r := recover(); r != nil {
				ran = false // quarantined, the frame continues
			}
		}()
		cb()
		return true
	})
	defer facet.ClearCallbackRecoveryHook()

	s := store.NewValueStore("a")
	m := newValueTestMark(s)
	m.bind.apply = func(string) { panic("boom") }
	facet.Attach(m, facet.AttachContext{})
	s.Set("b") // must not panic out of the delivery
}

// A nil store makes the binding a no-op: attach and write are safe, Store()
// is nil.
func TestScalarBinding_nilStore_noOp(t *testing.T) {
	m := newValueTestMark(nil)
	facet.Attach(m, facet.AttachContext{})
	m.edit("x") // must not panic
	if m.bind.Store() != nil {
		t.Fatal("nil-store binding exposed a non-nil store")
	}
}

// The binding is generic over any comparable scalar.
func TestScalarBinding_comparableKinds(t *testing.T) {
	boolMark := &Core{}
	boolMark.RegisterRoles(boolMark)
	b := BindScalar(boolMark, store.NewValueStore(true), facet.DirtyProjection, nil)
	if !b.Store().Get() {
		t.Fatal("bool scalar: unexpected zero value")
	}
	intMark := &Core{}
	intMark.RegisterRoles(intMark)
	i := BindScalar(intMark, store.NewValueStore(7), facet.DirtyProjection, nil)
	if i.Store().Get() != 7 {
		t.Fatal("int scalar: unexpected zero value")
	}
}

// Store() exposes the live store the mark was constructed over — injected
// stores are bound, never copied.
func TestScalarBinding_storeAccessor_boundNotCopied(t *testing.T) {
	s := store.NewValueStore("x")
	m := newValueTestMark(s)
	if m.bind.Store() != s {
		t.Fatal("Store() must return the injected store itself, not a copy")
	}
}

// Consistency model: the runtime thread is the single writer (store.Set
// asserts it), so an external write and a user edit interleave — never race.
// Last-writer-wins converges the mark and the store to one of the two writes.
// Run under -race -count=10 (NFR-4) to check the interleaving's memory model.
func TestScalarBinding_interleavedWrites_converge(t *testing.T) {
	s := store.NewValueStore("")
	m := newValueTestMark(s)
	facet.Attach(m, facet.AttachContext{})

	for i := 0; i < 50; i++ {
		m.edit("user")
		s.Set("external")
		if got := m.mirror; got != s.Get() {
			t.Fatalf("iteration %d: mirror %q diverged from store %q", i, got, s.Get())
		}
		m.edit("user2")
		if s.Get() != "user2" {
			t.Fatalf("iteration %d: user edit did not write back: %q", i, s.Get())
		}
	}
}
