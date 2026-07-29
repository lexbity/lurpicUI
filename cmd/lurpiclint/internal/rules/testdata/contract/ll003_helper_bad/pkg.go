package ll003_helper_bad

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

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

func (c *Container) arrangeChildren(ctx facet.ArrangeContext, bounds gfx.Rect) {
	c.childA.layout.ArrangedBounds = gfx.RectFromXYWH(0, 0, 100, 200)
	c.childB.layout.ArrangedBounds = gfx.RectFromXYWH(100, 0, 100, 200)
}

func newContainer() *Container {
	c := &Container{
		childA: &Child{},
		childB: &Child{},
	}
	c.Facet = facet.NewFacet()
	c.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		c.arrangeChildren(ctx, bounds)
	}
	c.AddRole(&c.layout)
	c.Facet.AddChild(c.childA.Base())
	c.Facet.AddChild(c.childB.Base())
	return c
}
