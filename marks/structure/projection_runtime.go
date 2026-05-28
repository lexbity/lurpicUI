package structure

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/theme"
)

// ProjectionRuntime is the subset of Runtime capabilities needed by structure
// mark projection code. Using this typed interface instead of `runtime any`
// lets the compiler verify interface satisfaction at the wiring point.
type ProjectionRuntime interface {
	facet.RuntimeServices
	RootStyleContext() any
	FacetByID(id facet.FacetID) facet.FacetImpl
}

// resolveStyleContext looks up the nearest style context for a facet using
// a ProjectionRuntime. Returns nil if the runtime is nil or doesn't support
// style tree lookups.
func resolveStyleContext(rt ProjectionRuntime, id facet.FacetID) *theme.StyleContextStore {
	return theme.NearestStyleContext(rt, id)
}
