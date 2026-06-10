package leaf

type LeafPane struct {
	facet.Facet
	layout facet.LayoutRole
}

func newLeafPane() *LeafPane {
	p := &LeafPane{}
	p.Facet = facet.NewFacet()
	p.layout = facet.LayoutRole{
		OnMeasure: func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
			return facet.MeasureResult{Size: c.MaxSize}
		},
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			p.layout.ArrangedBounds = bounds
			p.layout.Constraints = facet.Constraints{MaxSize: gfx.Size{W: bounds.Width(), H: bounds.Height()}}
		},
	}
	p.AddRole(&p.layout)
	return p
}
