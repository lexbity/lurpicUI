package delegate

type DelegatingPane struct {
	facet.Facet
	layout facet.LayoutRole
	childA *ChildPane
	childB *ChildPane
}

type ChildPane struct {
	facet.Facet
	layout facet.LayoutRole
}

func newDelegatingPane() *DelegatingPane {
	p := &DelegatingPane{
		childA: &ChildPane{},
		childB: &ChildPane{},
	}
	p.Facet = facet.NewFacet()
	p.layout = facet.LayoutRole{
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			p.arrangeChildren(ctx, bounds)
		},
	}
	p.AddRole(&p.layout)
	return p
}

// arrangeChildren is a helper that arranges children.  The OnArrange lambda
// calls this helper; cross-function analysis (Phase 13+) would trace into
// this body.  Phase 4 analyses only the lambda body itself.
func (p *DelegatingPane) arrangeChildren(ctx facet.ArrangeContext, bounds gfx.Rect) {
	p.childA.layout.ArrangedBounds = gfx.RectFromXYWH(0, 0, 100, 200)
	p.childB.layout.ArrangedBounds = gfx.RectFromXYWH(100, 0, 100, 200)
}
