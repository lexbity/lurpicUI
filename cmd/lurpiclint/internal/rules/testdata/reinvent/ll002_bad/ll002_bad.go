package ll002_bad

type Pane struct {
	facet.Facet
	layout facet.LayoutRole
}

func newPane() *Pane {
	p := &Pane{}
	p.Facet = facet.NewFacet()
	p.layout = facet.LayoutRole{
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			// Single RectFromXYWH with non-trivial (computed) args.
			// 0 Arrange/Measure calls, 1 rect → NOT child-arranging,
			// but LL002 fires because args are computed.
			_ = gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, bounds.Width(), bounds.Height())
		},
	}
	p.AddRole(&p.layout)
	return p
}
