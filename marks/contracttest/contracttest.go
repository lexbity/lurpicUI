package contracttest

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/store"
)

// TB is the subset of testing.TB consumed by the assertions in this package.
// Using this interface instead of *testing.T allows meta-tests to capture
// assertion failures without terminating the test goroutine.
type TB interface {
	Helper()
	Fatalf(format string, args ...any)
	Logf(format string, args ...any)
}

type contractRuntime struct{}

func (contractRuntime) Schedule(j job.AnyJob)                                              {}
func (contractRuntime) CancelJob(id job.JobID)                                             {}
func (contractRuntime) Invalidate(id facet.FacetID, flags facet.DirtyFlags, source string) {}

// AssertValueSurvivesDispose verifies that a writable-value mark uses the
// caller's store (P1) — the value in the caller's store survives a
// dispose + rebuild cycle on the same store.
//
//	makeStore: create a fresh store (e.g. store.NewValueStore[T](initial)).
//	build:     construct the mark, wiring the store as its writable value.
//	interact:  drive the mark to mutate the value (e.g. toggle, SetState).
//
// interact MUST change the store value; otherwise the assertion is trivially
// vacuous. The meta-test (AssertMutationCheckFails) proves the helper has
// teeth against deliberately broken marks.
func AssertValueSurvivesDispose[T comparable](t TB,
	makeStore func() *store.ValueStore[T],
	build func(*store.ValueStore[T]) facet.FacetImpl,
	interact func(facet.FacetImpl),
) {
	t.Helper()
	ctx := facet.AttachContext{Runtime: contractRuntime{}}

	s := makeStore()
	initial := s.Get()

	m := build(s)
	facet.Attach(m, ctx)
	interact(m)

	if s.Get() == initial {
		t.Fatalf("AssertValueSurvivesDispose: interaction did not change the caller's store "+
			"(initial=%v, after interact=%v) — the mark is likely using its own store, not the caller's",
			initial, s.Get())
	}
	want := s.Get()

	facet.Dispose(m)

	m2 := build(s)
	facet.Attach(m2, ctx)
	if got := s.Get(); got != want {
		t.Fatalf("AssertValueSurvivesDispose: store value after dispose+rebuild = %v, want %v "+
			"(the value set during interaction was lost — the mark likely reinitialized the store)", got, want)
	}
	facet.Dispose(m2)
}

// AssertBindingNotSevered verifies that a Binding field backed by a caller
// store is not overwritten with a Const by the mark's interaction handler.
//
//	makeStore:    create the backing store.
//	build:        construct the mark, assigning marks.FromStore(s, dirty) to
//	              the Binding field under test.
//	driveAction:  trigger the interaction that should mutate state (dismiss,
//	              toggle, activate, etc.).
//	readBinding:  read the Binding field's current value via Get().
//
// After driveAction the assertion checks that the Binding still reads the
// same value as the underlying store — proving the field was not replaced
// with a marks.Const(...) that would orphan the caller's truth.
func AssertBindingNotSevered[T comparable](t TB,
	makeStore func() *store.ValueStore[T],
	build func(*store.ValueStore[T]) facet.FacetImpl,
	driveAction func(facet.FacetImpl),
	readBinding func(facet.FacetImpl) T,
) {
	t.Helper()
	ctx := facet.AttachContext{Runtime: contractRuntime{}}

	s := makeStore()
	m := build(s)
	facet.Attach(m, ctx)

	driveAction(m)

	bindingVal := readBinding(m)
	storeVal := s.Get()
	if bindingVal != storeVal {
		t.Fatalf("AssertBindingNotSevered: binding read = %v, store read = %v — "+
			"the binding was severed (likely overwritten with marks.Const)", bindingVal, storeVal)
	}
	facet.Dispose(m)
}
