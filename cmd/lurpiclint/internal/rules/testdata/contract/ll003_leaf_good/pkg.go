package ll003_leaf_good

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

type LeafPane struct {
	facet.Facet
	layout facet.LayoutRole
}

func newLeafPane() *LeafPane {
	p := &LeafPane{}
	p.Facet = facet.NewFacet()
	p.layout.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: c.MaxSize}
	}
	p.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		p.layout.ArrangedBounds = bounds
	}
	p.AddRole(&p.layout)
	return p
}
