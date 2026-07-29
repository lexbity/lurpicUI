package ll020_good

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
	// OK: public Arrange method, not OnArrange.
	child.LayoutRole().Arrange(ctx, bounds)
}

func newContainer() *Container {
	c := &Container{child: &Child{}}
	c.Facet = facet.NewFacet()
	c.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		// OK: reading field values, not writing.
		_ = c.layout.ArrangedBounds
		_ = c.layout.MeasuredSize
	}
	c.AddRole(&c.layout)
	c.Facet.AddChild(c.child.Base())
	return c
}
