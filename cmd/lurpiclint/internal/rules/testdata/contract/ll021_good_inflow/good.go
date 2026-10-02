package ll021_good_inflow

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/marks/feedback"
	"codeburg.org/lexbit/lurpicui/store"
)

// Hosting an ALREADY-CONSTRUCTED overlay mark via a field reference is a
// structural choice (the in-flow demonstration idiom) — the narrowed rule
// does not fire on field references.
type Root struct {
	facet.Facet
	dialog *feedback.Dialog
}

func newRoot() *Root {
	r := &Root{
		dialog: feedback.NewDialog("title", "body", nil, store.NewValueStore(false)),
	}
	r.Facet = facet.NewFacet()
	r.Facet.AddChild(r.dialog.Base())
	return r
}
