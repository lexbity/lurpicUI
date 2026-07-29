package ll021_good_layered

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/marks/feedback"
	"codeburg.org/lexbit/lurpicui/store"
)

type Child struct{ facet.Facet }

func newChild() *Child { return &Child{Facet: facet.NewFacet()} }

type Root struct {
	facet.Facet
	dialog *feedback.Dialog
	surface *Child
}

func newRoot() *Root {
	r := &Root{
		dialog: feedback.NewDialog("title", "body", nil, store.NewValueStore(false)),
		surface: newChild(),
	}
	r.Facet = facet.NewFacet()
	facet.AttachLayer(r, r.surface, facet.LayerAttachment{ZPriority: 100})
	r.Facet.AddChild(r.dialog.Base())
	return r
}
