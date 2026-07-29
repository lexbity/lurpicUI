package ll022_good_small

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/layout"
)

type Child struct{ facet.Facet }

func newChild() *Child { return &Child{Facet: facet.NewFacet()} }

type ChromeBar struct {
	facet.Facet
	col *layout.ColumnLayout
}

func newChromeBar() *ChromeBar {
	p := &ChromeBar{}
	p.Facet = facet.NewFacet()
	p.col = layout.NewColumnLayout()
	p.col.Add(layout.Flexible(newChild(), 1))
	p.col.Add(layout.Fixed(newChild()))
	p.col.Add(layout.Fixed(newChild()))
	p.Facet.AddChild(p.col.Base())
	return p
}
