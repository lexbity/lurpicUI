package facet

import "fmt"

// TestIDRegistry is the author-identity uniqueness surface (QP-7). The
// runtime owns the live map; facet calls it at the attach and dispose
// transitions. Register panics on a duplicate live TestID — fail-loud by
// contract: a silent duplicate would make automation intermittently target
// the wrong facet.
type TestIDRegistry interface {
	Register(testID string, id FacetID, context string)
	Release(testID string, id FacetID)
}

// registerTestID validates and registers the facet's TestID at attach. A
// collision panics naming both facets (Principle 11).
func (f *Facet) registerTestID(context string) {
	if f.testID == "" || f.testIDRegistry == nil {
		return
	}
	f.testIDRegistry.Register(f.testID, f.id, context)
}

// releaseTestID removes the facet's TestID from the live registry at dispose.
func (f *Facet) releaseTestID() {
	if f.testID == "" || f.testIDRegistry == nil {
		return
	}
	f.testIDRegistry.Release(f.testID, f.id)
}

func (f *Facet) bindTestIDRegistry(registry TestIDRegistry) { f.testIDRegistry = registry }

// SetTestID assigns the author identity used by LDP locators (QP-7). It MUST
// be called during construction: assignment after the Created state panics,
// mirroring the tree-mutation contract — a TestID landing after attach would
// not be registered and silently unaddressable.
func (f *Facet) SetTestID(testID string) {
	if f == nil {
		return
	}
	if f.state != StateCreated {
		panic(fmt.Sprintf("facet: SetTestID(%q) after attach (state %s) — set author identity during construction", testID, f.state))
	}
	f.testID = testID
}

// TestID returns the author identity, or "" when unset.
func (f *Facet) TestID() string {
	if f == nil {
		return ""
	}
	return f.testID
}
