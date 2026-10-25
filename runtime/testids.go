package runtime

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/facet"
)

// liveTestIDs is the runtime's author-identity registry (QP-7): TestID →
// facet ID for every attached facet carrying one. Runtime-thread owned —
// registration happens in the attach transition, release in dispose, both on
// the runtime thread (dispose takes frameMu's write lock). Implements
// facet.TestIDRegistry.
type liveTestIdentity struct {
	id      facet.FacetID
	context string
}

type liveTestIDs map[string]liveTestIdentity

func (m liveTestIDs) Register(testID string, id facet.FacetID, context string) {
	if prev, exists := m[testID]; exists {
		panic(fmt.Sprintf(
			"runtime: duplicate live TestID %q: facet %d at %s and facet %d at %s both claim it — author identities must be unique while attached (Principle 11)",
			testID, prev.id, prev.context, id, context))
	}
	m[testID] = liveTestIdentity{id: id, context: context}
}

func (m liveTestIDs) Release(testID string, id facet.FacetID) {
	if prev, exists := m[testID]; exists && prev.id == id {
		delete(m, testID)
	}
}

// TestID resolves an author identity to its live facet ID (used by the drive
// locator walk). Zero when unknown.
func (rt *Runtime) FacetIDForTestID(testID string) facet.FacetID {
	return rt.liveTestIDs[testID].id
}
