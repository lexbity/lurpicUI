package ll003_bad

type Container struct {
	facet.Facet
	layout facet.LayoutRole
	childA *Child
	childB *Child
}

type Child struct {
	facet.Facet
	layout facet.LayoutRole
}

func newContainer() *Container {
	c := &Container{
		childA: &Child{},
		childB: &Child{},
	}
	c.Facet = facet.NewFacet()
	c.layout = facet.LayoutRole{
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			c.childA.layout.Arrange(ctx, gfx.RectFromXYWH(0, 0, 100, 100))
			c.childB.layout.Arrange(ctx, gfx.RectFromXYWH(100, 0, 100, 100))
		},
	}
	c.AddRole(&c.layout)
	c.Facet.AddChild(c.childA.Base())
	c.Facet.AddChild(c.childB.Base())
	return c
}
