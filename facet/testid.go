package facet

import "fmt"

// TestIDRegistry is the author-identity uniqueness surface (QP-7). The
// runtime owns the live map; facet calls it at the attach and dispose
// transitions. Register panics on a duplicate live TestID — fail-loud by
// contract: a silent duplicate would make automation intermittently target
// the wrong facet.
type TestIDRegistry interface {
	Register(testID string, id FacetID)
	Release(testID string, id FacetID)
}

// testIDRegistry is the installed registry (runtime.New wires the runtime's
// live map). Package-level like SetCollectPhaseCheck: attach transitions are
// runtime-thread events and the registry is installed before any tree
// attaches.
var testIDRegistry TestIDRegistry

// SetTestIDRegistry installs the live TestID registry. Called by the runtime
// constructor; reinstalling replaces the previous registry (last-constructed
// runtime wins — the same contract as store.SetSignalQueueHook and
// facet.SetCollectPhaseCheck, which test processes exercise with many
// runtimes).
func SetTestIDRegistry(r TestIDRegistry) {
	testIDRegistry = r
}

// ResetTestIDRegistryForTest clears the registry (test seams only).
func ResetTestIDRegistryForTest() { testIDRegistry = nil }

// registerTestID validates and registers the facet's TestID at attach. A
// collision panics naming both facets (Principle 11).
func (f *Facet) registerTestID() {
	if f.testID == "" || testIDRegistry == nil {
		return
	}
	testIDRegistry.Register(f.testID, f.id)
}

// releaseTestID removes the facet's TestID from the live registry at dispose.
func (f *Facet) releaseTestID() {
	if f.testID == "" || testIDRegistry == nil {
		return
	}
	testIDRegistry.Release(f.testID, f.id)
}

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
