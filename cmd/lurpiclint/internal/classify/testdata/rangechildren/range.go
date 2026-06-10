package rangechildren

type Parent struct {
	facet.Facet
	layout   facet.LayoutRole
	Children []*ChildPane
}

type ChildPane struct {
	facet.Facet
	layout facet.LayoutRole
}

func newParent() *Parent {
	p := &Parent{}
	p.Facet = facet.NewFacet()
	p.layout = facet.LayoutRole{
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			for _, child := range p.Children {
				child.layout.ArrangedBounds = gfx.RectFromXYWH(0, 0, 100, 50)
			}
		},
	}
	p.AddRole(&p.layout)
	return p
}
