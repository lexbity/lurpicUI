package root

type RootFacet struct {
	facet.Facet
	layout facet.LayoutRole
	childA *ChildPane
	childB *ChildPane
}

type ChildPane struct {
	facet.Facet
	layout facet.LayoutRole
}

func newRootFacet() *RootFacet {
	r := &RootFacet{
		childA: &ChildPane{},
		childB: &ChildPane{},
	}
	r.Facet = facet.NewFacet()
	r.layout = facet.LayoutRole{
		OnMeasure: func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
			r.childA.layout.Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: 100, H: 100}})
			r.childB.layout.Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: 100, H: 100}})
			return facet.MeasureResult{Size: gfx.Size{W: 200, H: 100}}
		},
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			r.childA.layout.Arrange(ctx, gfx.RectFromXYWH(0, 0, 100, 200))
			r.childB.layout.Arrange(ctx, gfx.RectFromXYWH(100, 0, 100, 200))
		},
	}
	r.AddRole(&r.layout)
	return r
}
