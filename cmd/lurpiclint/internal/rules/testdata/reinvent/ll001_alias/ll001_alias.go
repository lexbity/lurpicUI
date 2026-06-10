package ll001_alias

import (
	f "codeburg.org/lexbit/lurpicui/facet"
)

type AliasFacet struct {
	f.Facet
	layout f.LayoutRole
}

func newAliasBad() *AliasFacet {
	a := &AliasFacet{}
	a.Facet = f.NewFacet()
	a.layout = f.LayoutRole{
		OnMeasure: func(ctx f.MeasureContext, c f.Constraints) f.MeasureResult {
			return f.MeasureResult{Size: c.MaxSize}
		},
	}
	a.AddRole(&a.layout)
	return a
}
