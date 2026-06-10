package ll002_leaf_suppressed

type Canvas struct {
	facet.Facet
	layout facet.LayoutRole
}

func newCanvas() *Canvas {
	c := &Canvas{}
	c.Facet = facet.NewFacet()
	c.layout = facet.LayoutRole{
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			// Single RectFromXYWH with non-trivial (computed) args.
			// This is a leaf (no child arranging) but uses computed coords.
			// LL002 reports this: computed coords in a leaf.
			_ = gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, bounds.Width(), bounds.Height())
		},
	}
	c.AddRole(&c.layout)
	return c
}
