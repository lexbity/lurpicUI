package mock

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

type GoodLayout struct {
	role facet.LayoutRole
}

func newGoodLayout() *GoodLayout {
	g := &GoodLayout{}
	// LayoutRole callback assignment inside layout/ package is allowed.
	g.role = facet.LayoutRole{
		OnMeasure: func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
			return facet.MeasureResult{Size: c.MaxSize}
		},
	}
	return g
}
