package ll001_bad

type BadFacet struct {
	facet.Facet
	layout facet.LayoutRole
}

func newBad() *BadFacet {
	f := &BadFacet{}
	f.Facet = facet.NewFacet()
	f.layout = facet.LayoutRole{
		OnMeasure: func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
			return facet.MeasureResult{Size: c.MaxSize}
		},
	}
	f.AddRole(&f.layout)
	return f
}

func newBadImmediate() {
	_ = facet.LayoutRole{
		OnMeasure: nil,
		OnArrange: nil,
	}
	_ = facet.LayoutRole{
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {},
	}
}
