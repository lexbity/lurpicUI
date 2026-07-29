package ll022_good_scroll

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks/structure"
)

type Child struct{ facet.Facet }

func newChild() *Child { return &Child{Facet: facet.NewFacet()} }

type SourcesPanel struct {
	facet.Facet
	col     *layout.ColumnLayout
	scroll  *structure.ScrollRegion
}

func newSourcesPanel() *SourcesPanel {
	p := &SourcesPanel{}
	p.Facet = facet.NewFacet()
	p.col = layout.NewColumnLayout()
	for i := 0; i < 11; i++ {
		p.col.Add(layout.Fixed(newChild()))
	}
	p.scroll = structure.NewScrollRegion("")
	p.scroll.SetChildren([]structure.ScrollRegionChild{
		{Facet: p.col},
	})
	p.Facet.AddChild(p.scroll.Base())
	return p
}
