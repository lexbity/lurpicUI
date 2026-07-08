package ll001_package_bad

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

type BadFacet struct {
	facet.Facet
	layout facet.LayoutRole
}

func newBad() *BadFacet {
	b := &BadFacet{}
	b.layout = facet.LayoutRole{
		OnMeasure: func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
			return facet.MeasureResult{Size: c.MaxSize}
		},
	}
	return b
}
