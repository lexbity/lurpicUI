package ll002_good

type Leaf struct {
	facet.Facet
	layout facet.LayoutRole
}

func newLeaf() *Leaf {
	l := &Leaf{}
	l.Facet = facet.NewFacet()
	l.layout = facet.LayoutRole{
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			// Single RectFromXYWH with constant args, not in a loop
			// -> suppressed (legitimate leaf drawing).
			_ = gfx.RectFromXYWH(0, 0, 100, 100)
		},
	}
	l.AddRole(&l.layout)
	return l
}
