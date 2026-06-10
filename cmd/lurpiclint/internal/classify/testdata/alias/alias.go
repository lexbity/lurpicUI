package alias

import (
	f "codeburg.org/lexbit/lurpicui/facet"
	gg "codeburg.org/lexbit/lurpicui/gfx"
)

type AliasRoot struct {
	f.Facet
	layout f.LayoutRole
	childA *AliasChild
	childB *AliasChild
}

type AliasChild struct {
	f.Facet
	layout f.LayoutRole
}

func newAliasRoot() *AliasRoot {
	r := &AliasRoot{
		childA: &AliasChild{},
		childB: &AliasChild{},
	}
	r.Facet = f.NewFacet()
	r.layout = f.LayoutRole{
		OnMeasure: func(ctx f.MeasureContext, c f.Constraints) f.MeasureResult {
			r.childA.layout.Measure(ctx, f.Constraints{MaxSize: gg.Size{W: 100, H: 100}})
			r.childB.layout.Measure(ctx, f.Constraints{MaxSize: gg.Size{W: 100, H: 100}})
			return f.MeasureResult{Size: gg.Size{W: 200, H: 100}}
		},
		OnArrange: func(ctx f.ArrangeContext, bounds gg.Rect) {
			r.childA.layout.Arrange(ctx, gg.RectFromXYWH(0, 0, 100, 200))
			r.childB.layout.Arrange(ctx, gg.RectFromXYWH(100, 0, 100, 200))
		},
	}
	r.AddRole(&r.layout)
	return r
}
