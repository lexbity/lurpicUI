package ll012_good

import (
	"codeburg.org/lexbit/lurpicui/facet"
)

// Narrowed LL012: facet-composition slices, lifecycle closures, and cached
// projection state are not domain state.
type CompositionFacet struct {
	facet.Facet
	items    []facet.FacetImpl
	cleanups []func()
	children []*facet.Facet
}
