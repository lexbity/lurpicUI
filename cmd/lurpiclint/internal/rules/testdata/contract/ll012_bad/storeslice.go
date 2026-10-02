package ll012_bad

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/store"
)

// A slice of stores IS domain state.
type StoresFacet struct {
	facet.Facet
	binds []*store.ValueStore[string]
}
