package ll022_bad

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/layout"
)

type Child struct{ facet.Facet }

func newChild() *Child { return &Child{Facet: facet.NewFacet()} }

type Inspector struct {
	facet.Facet
	col *layout.ColumnLayout
}

func newInspector() *Inspector {
	p := &Inspector{}
	p.Facet = facet.NewFacet()
	p.col = layout.NewColumnLayout()
	for i := 0; i < 11; i++ {
		p.col.Add(layout.Fixed(newChild()))
	}
	p.Facet.AddChild(p.col.Base())
	return p
}
