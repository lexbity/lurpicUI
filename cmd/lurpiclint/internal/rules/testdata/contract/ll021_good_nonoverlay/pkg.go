package ll021_good_nonoverlay

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/layout"
)

type Child struct{ facet.Facet }

func newChild() *Child { return &Child{Facet: facet.NewFacet()} }

type Root struct {
	facet.Facet
	col *layout.ColumnLayout
}

func newRoot() *Root {
	r := &Root{}
	r.Facet = facet.NewFacet()
	r.col = layout.NewColumnLayout()
	r.col.Add(layout.Fixed(newChild()))
	r.Facet.AddChild(r.col.Base())
	return r
}
