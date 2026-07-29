package ll022_good_flex

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/layout"
)

type Child struct{ facet.Facet }

func newChild() *Child { return &Child{Facet: facet.NewFacet()} }

type Panel struct {
	facet.Facet
	col *layout.ColumnLayout
}

func newPanel() *Panel {
	p := &Panel{}
	p.Facet = facet.NewFacet()
	p.col = layout.NewColumnLayout()
	p.col.Add(layout.Fixed(newChild()))
	p.col.Add(layout.Flexible(newChild(), 1))
	p.Facet.AddChild(p.col.Base())
	return p
}
