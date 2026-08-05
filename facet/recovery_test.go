package facet

import (
	"sync"
	"testing"
)

// facetRecoveryStub mirrors the runtime's facet-recovery contract so focus
// recovery can be tested in isolation (facet cannot import runtime without an
// import cycle).
type facetRecoveryStub struct {
	mu       sync.RWMutex
	poisoned map[FacetID]struct{}
}

func (r *facetRecoveryStub) hook() CallbackRecoveryHook {
	return func(id FacetID, role string, cb func()) bool {
		if r.isPoisoned(id) {
			return false
		}
		defer func() {
			if p := recover(); p != nil {
				r.poison(id)
			}
		}()
		cb()
		return !r.isPoisoned(id)
	}
}

func (r *facetRecoveryStub) poison(id FacetID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.poisoned == nil {
		r.poisoned = make(map[FacetID]struct{})
	}
	r.poisoned[id] = struct{}{}
}

func (r *facetRecoveryStub) isPoisoned(id FacetID) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.poisoned == nil {
		return false
	}
	_, ok := r.poisoned[id]
	return ok
}

// panicFocusTestFacet is a focusable facet whose gained/lost callbacks record
// calls and panic independently.
type panicFocusTestFacet struct {
	Facet
	focus       FocusRole
	gainedCalls int
	lostCalls   int
	panicGained bool
	panicLost   bool
}

func (f *panicFocusTestFacet) Base() *Facet               { return &f.Facet }
func (f *panicFocusTestFacet) OnAttach(ctx AttachContext) {}
func (f *panicFocusTestFacet) OnDetach()                  {}
func (f *panicFocusTestFacet) OnActivate()                {}
func (f *panicFocusTestFacet) OnDeactivate()              {}

func newPanicFocusTestFacet(panicGained, panicLost bool) *panicFocusTestFacet {
	f := &panicFocusTestFacet{Facet: NewFacet(), panicGained: panicGained, panicLost: panicLost}
	f.focus.Focusable = func() bool { return true }
	f.focus.OnFocusGained = func() {
		f.gainedCalls++
		if f.panicGained {
			panic("boom")
		}
	}
	f.focus.OnFocusLost = func() {
		f.lostCalls++
		if f.panicLost {
			panic("boom")
		}
	}
	f.AddRole(&f.focus)
	return f
}

func TestFocus_PanicInOnFocusGained_Quarantined(t *testing.T) {
	rec := &facetRecoveryStub{}
	SetCallbackRecoveryHook(rec.hook())
	defer ClearCallbackRecoveryHook()

	a := newPanicFocusTestFacet(true, false)
	m := NewFocusManager()

	if !m.SetFocus(a) {
		t.Fatal("expected SetFocus to succeed")
	}
	if !rec.isPoisoned(a.Base().ID()) {
		t.Fatal("expected facet to be quarantined after OnFocusGained panic")
	}
	if a.gainedCalls != 1 {
		t.Fatalf("OnFocusGained invoked %d times, want 1", a.gainedCalls)
	}

	// A quarantined facet must not fire focus callbacks.
	m.ClearFocus()
	if a.lostCalls != 0 {
		t.Fatalf("quarantined facet OnFocusLost invoked %d times, want 0", a.lostCalls)
	}
	if !m.SetFocus(a) {
		t.Fatal("expected SetFocus to succeed")
	}
	if a.gainedCalls != 1 {
		t.Fatalf("OnFocusGained invoked %d times after quarantine, want 1 (poisoned facet skipped)", a.gainedCalls)
	}
}

func TestFocus_PanicInOnFocusLost_Quarantined(t *testing.T) {
	rec := &facetRecoveryStub{}
	SetCallbackRecoveryHook(rec.hook())
	defer ClearCallbackRecoveryHook()

	a := newPanicFocusTestFacet(false, true)
	b := newFocusTestFacet(0, true)
	m := NewFocusManager()

	if !m.SetFocus(a) {
		t.Fatal("expected SetFocus(a) to succeed")
	}
	if !m.SetFocus(b) {
		t.Fatal("expected SetFocus(b) to succeed")
	}
	if !rec.isPoisoned(a.Base().ID()) {
		t.Fatal("expected facet to be quarantined after OnFocusLost panic")
	}
	if a.lostCalls != 1 {
		t.Fatalf("OnFocusLost invoked %d times, want 1", a.lostCalls)
	}
	if b.gained != 1 {
		t.Fatalf("B OnFocusGained invoked %d times, want 1", b.gained)
	}

	// Re-focusing the quarantined facet must not re-invoke its callbacks.
	if !m.SetFocus(a) {
		t.Fatal("expected SetFocus(a) to succeed")
	}
	if a.gainedCalls != 1 {
		t.Fatalf("quarantined facet OnFocusGained invoked %d times, want 1 (unchanged)", a.gainedCalls)
	}
}

func TestCallbackRecoveryHook_NoHook_PanicPropagates(t *testing.T) {
	ClearCallbackRecoveryHook()

	a := newPanicFocusTestFacet(true, false)
	m := NewFocusManager()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected the panic to propagate with no recovery hook")
		}
	}()
	_ = m.SetFocus(a)
}
