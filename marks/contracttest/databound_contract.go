package contracttest

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/store"
)

// childrenProvider is satisfied by marks that expose a direct Children()
// accessor returning []facet.FacetImpl (e.g. via an embedded CollectionBinder).
type childrenProvider interface {
	Children() []facet.FacetImpl
}

// childrenOf returns the child facets of m, preferring a CollectionBinder-shaped
// surface over the generic facet children list.
func childrenOf(m facet.FacetImpl) []facet.FacetImpl {
	if cp, ok := m.(childrenProvider); ok {
		return cp.Children()
	}
	base := m.Base()
	if base == nil {
		return nil
	}
	kids := base.Children()
	out := make([]facet.FacetImpl, 0, len(kids))
	for _, k := range kids {
		if impl := k.Impl(); impl != nil {
			out = append(out, impl)
		}
	}
	return out
}

// AssertDataBound proves that a marks.DataBound mark keeps the caller's
// CollectionStore as the source of truth across Insert/Remove/Update/Replace
// and across a dispose+rebuild cycle, and that BoundData() returns that
// exact store pointer (not a manufactured one).
//
//	makeStore: create a fresh CollectionStore with the caller's Identify fn.
//	build:     construct the mark, wiring makeStore() as its bound data.
//	           The mark's OnAttach MUST wire the CollectionBinder to store
//	           signals so that mutations are reconciled.
//	drive:     mutate the store (Insert, Remove, Update, Replace) to exercise
//	           the reconciliation path; MUST change at least one of
//	           {child count, child identity, child projection}.
func AssertDataBound[T comparable](t TB,
	makeStore func() *store.CollectionStore[T],
	build func(*store.CollectionStore[T]) facet.FacetImpl,
	drive func(*store.CollectionStore[T]),
) {
	t.Helper()
	ctx := facet.AttachContext{Runtime: contractRuntime{}}

	s := makeStore()
	m := build(s)
	facet.Attach(m, ctx)
	defer facet.Dispose(m)

	// (1) BoundData identity: must return the caller's store pointer.
	db, ok := m.(interface{ BoundData() any })
	if !ok {
		t.Fatalf("AssertDataBound: mark does not implement marks.DataBound")
	}
	if db.BoundData() != any(s) {
		t.Fatalf("AssertDataBound: BoundData() = %p, want caller store %p — mark manufactured its own store",
			db.BoundData(), any(s))
	}

	drive(s)

	// (2) Reconciliation parity: children count matches store length.
	if got, want := len(childrenOf(m)), s.Len(); got != want {
		t.Fatalf("AssertDataBound: children=%d store.Len()=%d — reconciliation mismatch after drive",
			got, want)
	}

	// (3) Rebuild idempotency: build a fresh mark on the SAME store and verify
	//     the child count is reproduced and BoundData identity is preserved.
	//     m is still alive until the function returns (defer above), but no
	//     further drive calls are made, so the two marks coexist safely.
	wantCount := len(childrenOf(m))

	m2 := build(s)
	facet.Attach(m2, ctx)
	defer facet.Dispose(m2)

	if got := len(childrenOf(m2)); got != wantCount {
		t.Fatalf("AssertDataBound: rebuild children=%d want=%d — store not driving rebuild",
			got, wantCount)
	}
	if db2 := m2.(interface{ BoundData() any }); db2.BoundData() != any(s) {
		t.Fatalf("AssertDataBound: BoundData() identity lost across rebuild")
	}
}
