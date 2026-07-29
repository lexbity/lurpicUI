package ll003_field_assign_bad

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

func arrangeChildAtCtx(child *Child, rect gfx.Rect, ctx facet.ArrangeContext) {
	child.layout.OnArrange(ctx, rect)
}

func newContainer() *Container {
	c := &Container{
		childA: &Child{},
		childB: &Child{},
	}
	c.Facet = facet.NewFacet()
	c.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		arrangeChildAtCtx(c.childA, gfx.Rect{Min: gfx.Point{X: 0, Y: 0}, Max: gfx.Point{X: 100, Y: 100}}, ctx)
		arrangeChildAtCtx(c.childB, gfx.Rect{Min: gfx.Point{X: 100, Y: 0}, Max: gfx.Point{X: 200, Y: 100}}, ctx)
	}
	c.AddRole(&c.layout)
	c.Facet.AddChild(c.childA.Base())
	c.Facet.AddChild(c.childB.Base())
	return c
}
