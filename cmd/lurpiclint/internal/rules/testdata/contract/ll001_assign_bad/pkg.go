package ll001_assign_bad

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

// BadFacet carries a raw facet.LayoutRole field; the constructor populates
// its callbacks via field assignment rather than composition.  LL001 must
// flag both the OnMeasure and OnArrange assignments.
type BadFacet struct {
	facet.Facet
	layout facet.LayoutRole
	render facet.RenderRole
}

func newBad() *BadFacet {
	b := &BadFacet{}

	// LL001: field-assignment pattern outside layout/ or marks/.
	b.layout.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: c.MaxSize}
	}

	// LL001: second finding, distinct source position.
	b.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		b.layout.ArrangedBounds = bounds
	}

	b.Facet.AddRole(&b.layout)
	b.Facet.AddRole(&b.render)
	return b
}
