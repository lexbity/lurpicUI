package one_arrange_one_rect

type MixedPlacer struct {
	facet.Facet
	layout facet.LayoutRole
	childA *ChildPane
	childB *ChildPane
}

type ChildPane struct {
	facet.Facet
	layout facet.LayoutRole
}

func newMixedPlacer() *MixedPlacer {
	r := &MixedPlacer{
		childA: &ChildPane{},
		childB: &ChildPane{},
	}
	r.Facet = facet.NewFacet()
	r.layout = facet.LayoutRole{
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			// 1 Arrange call + 1 RectFromXYWH → branch 4
			r.childA.layout.Arrange(ctx, gfx.RectFromXYWH(0, 0, 100, 200))
		},
	}
	r.AddRole(&r.layout)
	return r
}
