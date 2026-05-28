package facet

import (
	"strings"
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

// --- AddChild panics ---

func TestFacetTree_addchild_nil_parent_panics(t *testing.T) {
	mustPanicContains(t, "nil parent in AddChild", func() {
		var f *Facet
		child := NewFacet()
		f.AddChild(&child)
	})
}

func TestFacetTree_addchild_nil_child_panics(t *testing.T) {
	mustPanicContains(t, "nil child in AddChild", func() {
		f := NewFacet()
		f.AddChild(nil)
	})
}

func TestFacetTree_addchild_self_panics(t *testing.T) {
	f := NewFacet()
	mustPanicContains(t, "cannot add facet as its own child", func() {
		f.AddChild(&f)
	})
}

func TestFacetTree_addchild_cycle_panics(t *testing.T) {
	a := NewFacet()
	b := NewFacet()
	a.AddChild(&b)
	mustPanicContains(t, "would create a cycle", func() {
		b.AddChild(&a)
	})
}

func TestFacetTree_addchild_already_has_parent_panics(t *testing.T) {
	a := NewFacet()
	b := NewFacet()
	c := NewFacet()
	a.AddChild(&c)
	mustPanicContains(t, "child already has a parent", func() {
		b.AddChild(&c)
	})
}

// --- AddChildRuntime panics ---

func TestFacetTree_addchild_runtime_nil_parent_panics(t *testing.T) {
	mustPanicContains(t, "nil parent in AddChildRuntime", func() {
		var f *Facet
		child := NewFacet()
		f.AddChildRuntime(&child)
	})
}

func TestFacetTree_addchild_runtime_nil_child_panics(t *testing.T) {
	f := NewFacet()
	mustPanicContains(t, "nil child in AddChildRuntime", func() {
		f.AddChildRuntime(nil)
	})
}

func TestFacetTree_addchild_runtime_self_panics(t *testing.T) {
	f := NewFacet()
	mustPanicContains(t, "cannot add facet as its own child", func() {
		f.AddChildRuntime(&f)
	})
}

func TestFacetTree_addchild_runtime_cycle_panics(t *testing.T) {
	a := NewFacet()
	b := NewFacet()
	a.AddChildRuntime(&b)
	mustPanicContains(t, "would create a cycle", func() {
		b.AddChildRuntime(&a)
	})
}

func TestFacetTree_addchild_runtime_already_has_parent_panics(t *testing.T) {
	a := NewFacet()
	b := NewFacet()
	c := NewFacet()
	a.AddChildRuntime(&c)
	mustPanicContains(t, "child already has a parent", func() {
		b.AddChildRuntime(&c)
	})
}

func TestFacetTree_addchild_runtime_on_disposed_parent_panics(t *testing.T) {
	f := newTestFacet()
	Attach(f, AttachContext{})
	Activate(f)
	Deactivate(f)
	Dispose(f)
	child := NewFacet()
	mustPanicContains(t, "AddChildRuntime on disposed parent", func() {
		f.AddChildRuntime(&child)
	})
}

func TestFacetTree_addchild_runtime_requires_created_child_panics(t *testing.T) {
	f := newTestFacet()
	Attach(f, AttachContext{})
	child := NewFacet()
	// Manually set child state to something other than Created
	child.setState(StateAttached)
	mustPanicContains(t, "requires a created child", func() {
		f.AddChildRuntime(&child)
	})
}

// --- RemoveChild panics ---

func TestFacetTree_removechild_nil_parent_panics(t *testing.T) {
	mustPanicContains(t, "nil parent in RemoveChild", func() {
		var f *Facet
		child := NewFacet()
		f.RemoveChild(&child)
	})
}

func TestFacetTree_removechild_non_child_panics(t *testing.T) {
	parent := NewFacet()
	child := NewFacet()
	mustPanicContains(t, "RemoveChild called for non-child", func() {
		parent.RemoveChild(&child)
	})
}

// --- Invalidate panics ---

func TestFacet_invalidate_nil_facet_panics(t *testing.T) {
	mustPanicContains(t, "nil facet in Invalidate", func() {
		var f *Facet
		f.Invalidate(DirtyLayout)
	})
}

func TestFacet_invalidate_after_dispose_panics(t *testing.T) {
	f := newTestFacet()
	Attach(f, AttachContext{})
	Activate(f)
	Deactivate(f)
	Dispose(f)
	mustPanicContains(t, "Invalidate after dispose", func() {
		f.Invalidate(DirtyLayout)
	})
}

// --- ClearDirty panics ---

func TestFacet_cleardirty_nil_facet_panics(t *testing.T) {
	mustPanicContains(t, "nil facet in ClearDirty", func() {
		var f *Facet
		f.ClearDirty(DirtyLayout)
	})
}

func TestFacet_cleardirty_after_dispose_panics(t *testing.T) {
	f := newTestFacet()
	Attach(f, AttachContext{})
	Activate(f)
	Deactivate(f)
	Dispose(f)
	mustPanicContains(t, "ClearDirty after dispose", func() {
		f.ClearDirty(DirtyLayout)
	})
}

// --- AddRole panics ---

func TestFacet_addrole_nil_facet_panics(t *testing.T) {
	mustPanicContains(t, "nil facet in AddRole", func() {
		var f *Facet
		f.AddRole(&testRole{})
	})
}

func TestFacet_addrole_nil_role_panics(t *testing.T) {
	f := NewFacet()
	mustPanicContains(t, "nil role in AddRole", func() {
		f.AddRole(nil)
	})
}

func TestFacet_addrole_after_attach_panics(t *testing.T) {
	f := newTestFacet()
	Attach(f, AttachContext{})
	mustPanicContains(t, "AddRole after attach", func() {
		f.AddRole(&testRole{})
	})
}

func TestFacet_addrole_duplicate_panics(t *testing.T) {
	f := NewFacet()
	r := &testRole{}
	f.AddRole(r)
	mustPanicContains(t, "duplicate role registration", func() {
		f.AddRole(r)
	})
}

// --- BindImpl panics ---

func TestFacet_bindimpl_different_impl_panics(t *testing.T) {
	f := NewFacet()
	f.BindImpl(newTestFacet())
	mustPanicContains(t, "BindImpl called with a different implementation", func() {
		f.BindImpl(newTestFacet())
	})
}

// --- nil receiver on exported methods ---

func mustPanicContains(t *testing.T, substr string, fn func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected panic containing %q, but no panic occurred", substr)
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic not a string: %T %v", r, r)
		}
		if !strings.Contains(msg, substr) {
			t.Fatalf("panic %q does not contain %q", msg, substr)
		}
	}()
	fn()
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
