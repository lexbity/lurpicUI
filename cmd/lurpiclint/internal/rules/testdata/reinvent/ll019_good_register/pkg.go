package ll019_good_register

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

type Child struct {
	facet.Facet
	layout facet.LayoutRole
}

type ParentWithRole struct {
	facet.Facet
	layout facet.LayoutRole
	child  *Child
}

func newParentWithRole() *ParentWithRole {
	p := &ParentWithRole{child: &Child{}}
	p.Facet = facet.NewFacet()
	p.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		p.layout.ArrangedBounds = bounds
	}
	p.Facet.AddRole(&p.layout)
	p.Facet.AddChild(p.child.Base())
	return p
}
