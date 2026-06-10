package multi_arrangedbounds

type BoundsPlacer struct {
	facet.Facet
	layout facet.LayoutRole
	childA *ChildPane
	childB *ChildPane
}

type ChildPane struct {
	facet.Facet
	layout facet.LayoutRole
}

func newBoundsPlacer() *BoundsPlacer {
	r := &BoundsPlacer{
		childA: &ChildPane{},
		childB: &ChildPane{},
	}
	r.Facet = facet.NewFacet()
	r.layout = facet.LayoutRole{
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			// 2 ArrangedBounds, 0 RectFromXYWH, 0 Arrange/Measure → branch 3
			r.childA.layout.ArrangedBounds = bounds
			r.childB.layout.ArrangedBounds = bounds
		},
	}
	r.AddRole(&r.layout)
	return r
}
