package contracttest

import (
	"codeburg.org/lexbit/lurpicui/facet"
)

// AssertGroupChildren proves that a mark exposing Children() []facet.GroupChild:
//  1. returns a non-empty child list with every GroupChild.Layout non-nil,
//  2. every child registered on the facet (Base().Children()) has a matching
//     entry in the GroupChild list (matched by FacetID),
//  3. after parent dispose every facet-registered child is disposed,
//  4. a fresh build with the same data reproduces the child count.
//
// build:   construct the mark for a fixed data state.
// fixDataState: re-apply the data state after rebuild so Children() is
//
//	derived from the same data. Marks that set data in their
//	constructor can pass a no-op closure.
func AssertGroupChildren(
	t TB,
	build func() facet.FacetImpl,
	fixDataState func(facet.FacetImpl),
) {
	t.Helper()
	ctx := facet.AttachContext{Runtime: contractRuntime{}}

	m := build()
	facet.Attach(m, ctx)
	fixDataState(m)

	// capturedBaseKids is populated after Children() is resolved below.
	// The disposal-check defer captures it as a closure variable, so it
	// will read the value set later when the defer executes.
	var capturedBaseKids []*facet.Facet

	// Nested defers: dispose-m runs FIRST (inner defer), then
	// the disposal check runs LAST (outer defer). LIFO ordering
	// guarantees m is disposed before children are inspected.
	defer func() {
		for _, cf := range capturedBaseKids {
			if !cf.IsDisposed() {
				t.Fatalf("AssertGroupChildren: child FacetID=%d not disposed after parent dispose",
					cf.ID())
			}
		}
	}()
	defer func() {
		facet.Dispose(m)
	}()

	// (1) Resolve the ChildSource interface.
	cs, ok := m.(facet.ChildSource)
	if !ok {
		t.Fatalf("AssertGroupChildren: mark does not expose Children() []facet.GroupChild " +
			"(not a facet.ChildSource)")
	}
	kids := cs.Children()
	if len(kids) == 0 {
		t.Fatalf("AssertGroupChildren: Children() returned empty after fixDataState")
	}
	for i, gc := range kids {
		if gc.Layout == nil {
			t.Fatalf("AssertGroupChildren: Children()[%d].Layout is nil", i)
		}
	}

	// (2) Verify every facet-registered child (Base().Children()) appears in
	// the GroupChild list by matching FacetID.
	baseKids := m.Base().Children()
	for _, cf := range baseKids {
		found := false
		for _, gc := range kids {
			if cf.ID() == gc.FacetID {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("AssertGroupChildren: base child FacetID=%d not represented in Children()",
				cf.ID())
		}
	}

	capturedBaseKids = baseKids
	wantCount := len(kids)

	// (3) Dispose happens via defer (see above).

	// (4) Rebuild with the same data state and verify the child count is reproduced.
	m2 := build()
	facet.Attach(m2, ctx)
	defer facet.Dispose(m2)
	fixDataState(m2)

	cs2, ok := m2.(facet.ChildSource)
	if !ok {
		t.Fatalf("AssertGroupChildren: rebuilt mark is not a facet.ChildSource")
	}
	if got := len(cs2.Children()); got != wantCount {
		t.Fatalf("AssertGroupChildren: rebuild children=%d want=%d", got, wantCount)
	}
}
