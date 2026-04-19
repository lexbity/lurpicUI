package facet

import "sync/atomic"

// FacetID is a stable, unique identity for a facet within the runtime.
type FacetID uint64

var idSource atomic.Uint64

// nextID returns a new non-zero facet identifier.
func nextID() FacetID {
	return FacetID(idSource.Add(1))
}
