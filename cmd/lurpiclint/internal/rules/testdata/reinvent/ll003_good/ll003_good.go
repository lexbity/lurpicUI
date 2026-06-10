package ll003_good

type Leaf struct {
	facet.Facet
	layout facet.LayoutRole
}

func newLeaf() *Leaf {
	l := &Leaf{}
	l.Facet = facet.NewFacet()
	l.layout = facet.LayoutRole{
		OnMeasure: func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
			return facet.MeasureResult{Size: c.MaxSize}
		},
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			l.layout.ArrangedBounds = bounds
		},
	}
	l.AddRole(&l.layout)
	return l
}
