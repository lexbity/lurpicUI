package mark

import (
	facet "codeburg.org/lexbit/lurpicui/facet"
)

// groupCore simulates marks.Core.
type groupCore struct {
	facet.Facet
	Layout facet.LayoutRole
}

// LeafMark is a container-shaped leaf: it embeds groupCore (so the fingerprint
// classifies it as a container via the promoted layout role and a child-slice
// field) but declares NO Children() method.  LL031 must not fire on it — the
// R-3 over-fire guard requires a container AND a Children() method.
type LeafMark struct {
	groupCore
	children []facet.FacetImpl
}

func NewLeafMark() *LeafMark {
	return &LeafMark{}
}
