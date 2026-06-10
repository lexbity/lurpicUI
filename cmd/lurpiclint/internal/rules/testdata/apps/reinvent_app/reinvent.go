package reinvent_app

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

// Container is a child-arranging facet — triggers LL003 (error) + LL001 (warn).
// LL002 is de-duplicated away because LL003 already fires on this LayoutRole.
type Container struct {
	facet.Facet
	layout facet.LayoutRole
	child  *Leaf
}

func newContainer() *Container {
	c := &Container{child: &Leaf{}}
	c.Facet = facet.NewFacet()
	c.layout = facet.LayoutRole{
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			c.child.layout.Arrange(ctx, gfx.RectFromXYWH(0, 0, 100, 100))
		},
	}
	c.AddRole(&c.layout)
	c.Facet.AddChild(c.child.Base())
	return c
}

// Leaf is a non-child-arranging facet with absolute coordinate placement —
// triggers LL002 (warn) because it uses RectFromXYWH with computed args
// but does not arrange children, so LL003 does not suppress LL002.
type Leaf struct {
	facet.Facet
	layout facet.LayoutRole
}

func newLeaf() *Leaf {
	l := &Leaf{}
	l.Facet = facet.NewFacet()
	l.layout = facet.LayoutRole{
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			_ = gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, bounds.Width(), bounds.Height())
		},
	}
	l.AddRole(&l.layout)
	return l
}
