package shapematch_app

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

// CustomPanel is a child-arranging facet that structurally resembles a
// built-in mark container — triggering LL004 (info) via fingerprint match.
type CustomPanel struct {
	facet.Facet
	layout facet.LayoutRole
	child  *CustomItem
}

type CustomItem struct {
	facet.Facet
}

func newCustomPanel() *CustomPanel {
	p := &CustomPanel{child: &CustomItem{}}
	p.Facet = facet.NewFacet()
	p.layout = facet.LayoutRole{
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			p.child.Arrange(ctx, gfx.RectFromXYWH(0, 0, bounds.Width(), bounds.Height()))
		},
	}
	p.AddRole(&p.layout)
	p.Facet.AddChild(p.child.Base())
	return p
}
