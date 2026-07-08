package ll018_good

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/marks/feedback"
)

type GoodFacet struct {
	facet.Facet
}

func newGoodFacet() *GoodFacet {
	g := &GoodFacet{}
	dialog := feedback.NewDialog("Title", "Message", nil)
	g.Facet.AddChild(dialog.Base())
	return g
}
