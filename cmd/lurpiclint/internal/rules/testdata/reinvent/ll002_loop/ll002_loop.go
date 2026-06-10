package ll002_loop

type LoopPane struct {
	facet.Facet
	layout facet.LayoutRole
	items  []float32
}

func newLoopPane() *LoopPane {
	p := &LoopPane{}
	p.Facet = facet.NewFacet()
	p.layout = facet.LayoutRole{
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			// Single RectFromXYWH inside a regular for loop (not range).
			// 0 Arrange/Measure calls, 1 rect in for loop → NOT
			// child-arranging, but LL002 fires because it's looped.
			for i := 0; i < len(p.items); i++ {
				_ = gfx.RectFromXYWH(0, float32(i)*20, 100, 20)
			}
		},
	}
	p.AddRole(&p.layout)
	return p
}
