package ll021_bad

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/marks/feedback"
	"codeburg.org/lexbit/lurpicui/store"
)

// The narrowed LL021 fires where an overlay is CONSTRUCTED and mounted as a
// plain sibling in the same expression.
func newRoot() *Root {
	r := &Root{}
	r.Facet = facet.NewFacet()
	r.Facet.AddChild(feedback.NewDialog("title", "body", nil, store.NewValueStore(false)).Base())
	return r
}

type Root struct {
	facet.Facet
}
