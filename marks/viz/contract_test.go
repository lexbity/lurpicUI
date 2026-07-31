package viz

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/contracttest"
	"codeburg.org/lexbit/lurpicui/scale/reactive"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme"
)

func newContractLinearScale() *reactive.ReactiveScale {
	domain := store.NewValueStore([2]float64{0, 100})
	rng := store.NewValueStore([2]float64{0, 200})
	return reactive.NewLinearReactive(domain, rng)
}

func TestAxis_contract_anchor_export(t *testing.T) {
	fonts := (axisGoldenRuntime{}).FontRegistry()
	rt := contracttest.NoopRuntime{}
	ctx := theme.DefaultResolvedContext()
	bounds := gfx.RectFromXYWH(0, 0, 720, 1400)

	contracttest.AssertAnchorExport(t,
		func() facet.FacetImpl {
			return NewAxis(newContractLinearScale(), marks.Const(AxisBottom), fonts)
		},
		func(m facet.FacetImpl, _ facet.AttachContext, b gfx.Rect) {
			a := m.(*Axis)
			a.Layout.Measure(facet.MeasureContext{
				Runtime:          rt,
				Theme:            ctx,
				ContentScale:     1,
				Density:          facet.DensityID(theme.DensityIDComfortable),
				WritingDirection: facet.WritingDirectionLTR,
			}, facet.Constraints{MaxSize: gfx.Size{W: b.Width(), H: b.Height()}})
			a.Layout.Arrange(facet.ArrangeContext{
				Runtime:     rt,
				Theme:       ctx,
				ParentGroup: a.Layout.Parent,
				ChildGroup:  a.Layout.Child,
			}, b)
		},
		bounds,
		ctx,
	)
}

func TestRule_contract_anchor_export(t *testing.T) {
	rt := contracttest.NoopRuntime{}
	ctx := theme.DefaultResolvedContext()
	bounds := gfx.RectFromXYWH(0, 0, 720, 1400)

	contracttest.AssertAnchorExport(t,
		func() facet.FacetImpl {
			return NewRule(marks.Const(50.0), RuleHorizontal, newContractLinearScale())
		},
		func(m facet.FacetImpl, _ facet.AttachContext, b gfx.Rect) {
			r := m.(*Rule)
			r.Layout.Measure(facet.MeasureContext{
				Runtime:          rt,
				Theme:            ctx,
				ContentScale:     1,
				Density:          facet.DensityID(theme.DensityIDComfortable),
				WritingDirection: facet.WritingDirectionLTR,
			}, facet.Constraints{MaxSize: gfx.Size{W: b.Width(), H: b.Height()}})
			r.Layout.Arrange(facet.ArrangeContext{
				Runtime:     rt,
				Theme:       ctx,
				ParentGroup: r.Layout.Parent,
				ChildGroup:  r.Layout.Child,
			}, b)
		},
		bounds,
		ctx,
	)
}
