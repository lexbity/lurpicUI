package ll020_write_bad

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

func arrangeChild(child *Child, bounds gfx.Rect) {
	// LL020: direct ArrangedBounds write bypasses LayoutRole.Arrange.
	child.layout.ArrangedBounds = bounds
}

func newContainer() *Container {
	c := &Container{child: &Child{}}
	c.Facet = facet.NewFacet()
	c.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		// LL020: direct ArrangedBounds write.
		c.layout.ArrangedBounds = bounds
	}
	c.AddRole(&c.layout)
	c.Facet.AddChild(c.child.Base())
	return c
}
