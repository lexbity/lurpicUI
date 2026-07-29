package ll021_bad

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/marks/feedback"
	"codeburg.org/lexbit/lurpicui/store"
)

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
