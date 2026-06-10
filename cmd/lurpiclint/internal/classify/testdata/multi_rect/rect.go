package multi_rect

type RectPlacer struct {
	facet.Facet
	layout facet.LayoutRole
	childA *ChildPane
	childB *ChildPane
}

type ChildPane struct {
	facet.Facet
	layout facet.LayoutRole
}

func newRectPlacer() *RectPlacer {
	r := &RectPlacer{
		childA: &ChildPane{},
		childB: &ChildPane{},
	}
	r.Facet = facet.NewFacet()
	r.layout = facet.LayoutRole{
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			// 2 RectFromXYWH, 0 Arrange/Measure calls → branch 2
			r.childA.layout.ArrangedBounds = gfx.RectFromXYWH(0, 0, 100, 200)
			r.childB.layout.ArrangedBounds = gfx.RectFromXYWH(100, 0, 100, 200)
		},
	}
	r.AddRole(&r.layout)
	return r
}
