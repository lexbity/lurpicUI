package ll020_call_bad

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

type Child struct {
	facet.Facet
	layout facet.LayoutRole
}

type Container struct {
	facet.Facet
	layout facet.LayoutRole
	child  *Child
}

func arrangeChild(child *Child, ctx facet.ArrangeContext, bounds gfx.Rect) {
	// LL020: direct OnArrange invocation bypasses LayoutRole.Arrange.
	child.layout.OnArrange(ctx, bounds)
}

func newContainer() *Container {
	c := &Container{child: &Child{}}
	c.Facet = facet.NewFacet()
	c.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		// LL020: direct OnArrange via LayoutRole() accessor.
		c.child.Base().LayoutRole().OnArrange(ctx, bounds)
	}
	c.AddRole(&c.layout)
	c.Facet.AddChild(c.child.Base())
	return c
}
