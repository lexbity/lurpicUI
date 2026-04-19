package facet

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/signal"
)

type testRole struct {
	attachCalled     int
	activateCalled   int
	deactivateCalled int
	disposeCalled    int
}

func (r *testRole) onAttach(f *Facet)     { r.attachCalled++ }
func (r *testRole) onActivate(f *Facet)   { r.activateCalled++ }
func (r *testRole) onDeactivate(f *Facet) { r.deactivateCalled++ }
func (r *testRole) onDispose(f *Facet)    { r.disposeCalled++ }

type testFacet struct {
	Facet

	attachCalled     int
	detachCalled     int
	activateCalled   int
	deactivateCalled int

	role *testRole
}

func newTestFacet() *testFacet {
	f := &testFacet{Facet: NewFacet()}
	f.role = &testRole{}
	f.AddRole(f.role)
	return f
}

func (f *testFacet) Base() *Facet               { return &f.Facet }
func (f *testFacet) OnAttach(ctx AttachContext) { f.attachCalled++ }
func (f *testFacet) OnDetach()                  { f.detachCalled++ }
func (f *testFacet) OnActivate()                { f.activateCalled++ }
func (f *testFacet) OnDeactivate()              { f.deactivateCalled++ }

func TestFacetID_unique_per_facet(t *testing.T) {
	a := NewFacet()
	b := NewFacet()
	if a.ID() == 0 || b.ID() == 0 {
		t.Fatalf("expected non-zero IDs, got %d and %d", a.ID(), b.ID())
	}
	if a.ID() == b.ID() {
		t.Fatalf("expected distinct IDs, got %d", a.ID())
	}
}

func TestFacetID_never_zero(t *testing.T) {
	f := NewFacet()
	if got := f.ID(); got == 0 {
		t.Fatal("expected non-zero facet ID")
	}
}

func TestLifecycle_created_state_on_construction(t *testing.T) {
	f := NewFacet()
	if got := f.State(); got != StateCreated {
		t.Fatalf("expected StateCreated, got %s", got)
	}
}

func TestLifecycle_attach_transitions_state(t *testing.T) {
	f := newTestFacet()
	Attach(f, AttachContext{})
	if got := f.State(); got != StateAttached {
		t.Fatalf("expected StateAttached, got %s", got)
	}
	if f.attachCalled != 1 {
		t.Fatalf("expected OnAttach once, got %d", f.attachCalled)
	}
	if f.role.attachCalled != 1 {
		t.Fatalf("expected role onAttach once, got %d", f.role.attachCalled)
	}
}

func TestLifecycle_activate_transitions_state(t *testing.T) {
	f := newTestFacet()
	Attach(f, AttachContext{})
	Activate(f)
	if got := f.State(); got != StateActive {
		t.Fatalf("expected StateActive, got %s", got)
	}
	if f.activateCalled != 1 {
		t.Fatalf("expected OnActivate once, got %d", f.activateCalled)
	}
	if f.role.activateCalled != 1 {
		t.Fatalf("expected role onActivate once, got %d", f.role.activateCalled)
	}
}

func TestLifecycle_deactivate_transitions_state(t *testing.T) {
	f := newTestFacet()
	Attach(f, AttachContext{})
	Activate(f)
	Deactivate(f)
	if got := f.State(); got != StateInactive {
		t.Fatalf("expected StateInactive, got %s", got)
	}
	if f.deactivateCalled != 1 {
		t.Fatalf("expected OnDeactivate once, got %d", f.deactivateCalled)
	}
	if f.role.deactivateCalled != 1 {
		t.Fatalf("expected role onDeactivate once, got %d", f.role.deactivateCalled)
	}
}

func TestLifecycle_reactivate_from_inactive(t *testing.T) {
	f := newTestFacet()
	Attach(f, AttachContext{})
	Activate(f)
	Deactivate(f)
	Activate(f)
	if got := f.State(); got != StateActive {
		t.Fatalf("expected StateActive, got %s", got)
	}
}

func TestLifecycle_dispose_from_active(t *testing.T) {
	f := newTestFacet()
	Attach(f, AttachContext{})
	Activate(f)
	Dispose(f)
	if got := f.State(); got != StateDisposed {
		t.Fatalf("expected StateDisposed, got %s", got)
	}
	if f.detachCalled != 1 {
		t.Fatalf("expected OnDetach once, got %d", f.detachCalled)
	}
	if f.role.disposeCalled != 1 {
		t.Fatalf("expected role onDispose once, got %d", f.role.disposeCalled)
	}
	if got := f.Subs().Len(); got != 0 {
		t.Fatalf("expected subscriptions released, got %d", got)
	}
}

func TestLifecycle_dispose_is_terminal(t *testing.T) {
	f := newTestFacet()
	Dispose(f)
	mustPanic(t, func() {
		Activate(f)
	})
}

func TestLifecycle_invalid_transition_panics(t *testing.T) {
	f := newTestFacet()
	mustPanic(t, func() {
		Activate(f)
	})
}

func TestFacetTree_addchild_sets_parent(t *testing.T) {
	parent := NewFacet()
	child := NewFacet()
	parent.AddChild(&child)
	if child.Parent() != &parent {
		t.Fatalf("expected parent pointer to be set")
	}
	if got := len(parent.Children()); got != 1 {
		t.Fatalf("expected 1 child, got %d", got)
	}
}

func TestFacetTree_addchild_after_attach_panics(t *testing.T) {
	parent := newTestFacet()
	Attach(parent, AttachContext{})
	child := NewFacet()
	mustPanic(t, func() {
		parent.AddChild(&child)
	})
}

func TestFacetTree_removechild_disposes(t *testing.T) {
	parent := NewFacet()
	child := NewFacet()
	parent.AddChild(&child)
	parent.RemoveChild(&child)
	if got := child.State(); got != StateDisposed {
		t.Fatalf("expected child disposed, got %s", got)
	}
	if child.Parent() != nil {
		t.Fatal("expected child parent cleared")
	}
}

func TestFacetDirty_flags_set_and_clear(t *testing.T) {
	f := NewFacet()
	f.Invalidate(DirtyProjection)
	if got := f.DirtyFlags(); got != DirtyProjection {
		t.Fatalf("expected DirtyProjection, got %s", got)
	}
	f.ClearDirty(DirtyProjection)
	if got := f.DirtyFlags(); got != 0 {
		t.Fatalf("expected no dirty flags, got %s", got)
	}
}

func TestFacetDirty_all_flag_sets_all(t *testing.T) {
	f := NewFacet()
	f.Invalidate(DirtyAll)
	if got := f.DirtyFlags(); got != DirtyAll {
		t.Fatalf("expected DirtyAll, got %s", got)
	}
}

func TestSubs_released_on_dispose(t *testing.T) {
	f := newTestFacet()
	sig := signal.NewSignal[signal.Unit]("test")
	called := 0
	To(Subscribe(f), &sig, func(signal.Unit) {
		called++
	})
	if got := f.Subs().Len(); got != 1 {
		t.Fatalf("expected one subscription, got %d", got)
	}
	Dispose(f)
	if got := f.Subs().Len(); got != 0 {
		t.Fatalf("expected subscriptions released, got %d", got)
	}
	sig.Emit(signal.Fired)
	if called != 0 {
		t.Fatalf("expected handler to be unsubscribed, got %d calls", called)
	}
}

func TestFacetSubs_attach_detach_reattach_clean(t *testing.T) {
	f := newTestFacet()
	sig := signal.NewSignal[signal.Unit]("test")
	To(Subscribe(f), &sig, func(signal.Unit) {})
	Attach(f, AttachContext{})
	Dispose(f)
	if got := f.Subs().Len(); got != 0 {
		t.Fatalf("expected released subscriptions, got %d", got)
	}
	next := newTestFacet()
	if got := next.Subs().Len(); got != 0 {
		t.Fatalf("expected fresh facet to start clean, got %d", got)
	}
}

func mustPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	fn()
}
