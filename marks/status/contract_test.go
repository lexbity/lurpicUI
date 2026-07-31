package status

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks/contracttest"
	"codeburg.org/lexbit/lurpicui/theme"
)

func TestBadge_contract_group_children(t *testing.T) {
	contracttest.AssertGroupChildren(t,
		func() facet.FacetImpl { return NewBadge("label") },
		func(facet.FacetImpl) {},
	)
}

func TestBadge_contract_anchor_export(t *testing.T) {
	bounds := gfx.RectFromXYWH(0, 0, 720, 1400)
	tokens := badgeTokens()
	rt := badgeRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, tokens, nil),
		fonts:     testkit.TestFontRegistry(t),
	}
	ctx := badgeResolvedContext(tokens, theme.DensityIDComfortable, layout.WritingDirectionLTR)
	contracttest.AssertAnchorExport(t,
		func() facet.FacetImpl { return NewBadge("label") },
		func(m facet.FacetImpl, _ facet.AttachContext, b gfx.Rect) {
			x := m.(*Badge)
			x.Layout.Measure(facet.MeasureContext{
				Runtime:          rt,
				Theme:            ctx,
				ContentScale:     1,
				Density:          facet.DensityID(theme.DensityIDComfortable),
				WritingDirection: facet.WritingDirectionLTR,
			}, facet.Constraints{MaxSize: gfx.Size{W: b.Width(), H: b.Height()}})
			x.Layout.Arrange(facet.ArrangeContext{
				Runtime:     rt,
				Theme:       ctx,
				ParentGroup: x.Layout.Parent,
				ChildGroup:  x.Layout.Child,
				Placement:   facet.Placement{Mode: facet.PlacementLinear},
			}, b)
		},
		bounds, ctx,
	)
}

func TestBadge_contract_accessible(t *testing.T) {
	contracttest.AssertAccessible(t,
		func(label string) facet.FacetImpl { return NewBadge(label) },
		"status",
	)
}

func TestProgressBar_contract_anchor_export(t *testing.T) {
	bounds := gfx.RectFromXYWH(0, 0, 720, 1400)
	tokens := badgeTokens()
	rt := badgeRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, tokens, nil),
		fonts:     testkit.TestFontRegistry(t),
	}
	ctx := badgeResolvedContext(tokens, theme.DensityIDComfortable, layout.WritingDirectionLTR)
	contracttest.AssertAnchorExport(t,
		func() facet.FacetImpl { return newProgressBarFixture() },
		func(m facet.FacetImpl, _ facet.AttachContext, b gfx.Rect) {
			x := m.(*ProgressBar)
			x.Layout.Measure(facet.MeasureContext{
				Runtime:          rt,
				Theme:            ctx,
				ContentScale:     1,
				Density:          facet.DensityID(theme.DensityIDComfortable),
				WritingDirection: facet.WritingDirectionLTR,
			}, facet.Constraints{MaxSize: gfx.Size{W: b.Width(), H: b.Height()}})
			x.Layout.Arrange(facet.ArrangeContext{
				Runtime:     rt,
				Theme:       ctx,
				ParentGroup: x.Layout.Parent,
				ChildGroup:  x.Layout.Child,
			}, b)
		},
		bounds, ctx,
	)
}

func TestProgressBar_contract_accessible(t *testing.T) {
	contracttest.AssertAccessible(t,
		func(label string) facet.FacetImpl { return NewProgressBar(label) },
		"progressbar",
	)
}

func TestProgressRing_contract_anchor_export(t *testing.T) {
	bounds := gfx.RectFromXYWH(0, 0, 720, 1400)
	tokens := badgeTokens()
	rt := badgeRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, tokens, nil),
		fonts:     testkit.TestFontRegistry(t),
	}
	ctx := badgeResolvedContext(tokens, theme.DensityIDComfortable, layout.WritingDirectionLTR)
	contracttest.AssertAnchorExport(t,
		func() facet.FacetImpl { return newProgressRingFixture() },
		func(m facet.FacetImpl, _ facet.AttachContext, b gfx.Rect) {
			x := m.(*ProgressRing)
			x.Layout.Measure(facet.MeasureContext{
				Runtime:          rt,
				Theme:            ctx,
				ContentScale:     1,
				Density:          facet.DensityID(theme.DensityIDComfortable),
				WritingDirection: facet.WritingDirectionLTR,
			}, facet.Constraints{MaxSize: gfx.Size{W: b.Width(), H: b.Height()}})
			x.Layout.Arrange(facet.ArrangeContext{
				Runtime:     rt,
				Theme:       ctx,
				ParentGroup: x.Layout.Parent,
				ChildGroup:  x.Layout.Child,
			}, b)
		},
		bounds, ctx,
	)
}

func TestProgressRing_contract_accessible(t *testing.T) {
	contracttest.AssertAccessible(t,
		func(label string) facet.FacetImpl { return NewProgressRing(label) },
		"progressbar",
	)
}

func TestStatusLight_contract_anchor_export(t *testing.T) {
	bounds := gfx.RectFromXYWH(0, 0, 720, 1400)
	tokens := badgeTokens()
	rt := badgeRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, tokens, nil),
		fonts:     testkit.TestFontRegistry(t),
	}
	ctx := badgeResolvedContext(tokens, theme.DensityIDComfortable, layout.WritingDirectionLTR)
	contracttest.AssertAnchorExport(t,
		func() facet.FacetImpl { return newStatusLightFixture() },
		func(m facet.FacetImpl, _ facet.AttachContext, b gfx.Rect) {
			x := m.(*StatusLight)
			x.Layout.Measure(facet.MeasureContext{
				Runtime:          rt,
				Theme:            ctx,
				ContentScale:     1,
				Density:          facet.DensityID(theme.DensityIDComfortable),
				WritingDirection: facet.WritingDirectionLTR,
			}, facet.Constraints{MaxSize: gfx.Size{W: b.Width(), H: b.Height()}})
			x.Layout.Arrange(facet.ArrangeContext{
				Runtime:     rt,
				Theme:       ctx,
				ParentGroup: x.Layout.Parent,
				ChildGroup:  x.Layout.Child,
			}, b)
		},
		bounds, ctx,
	)
}

func TestStatusLight_contract_accessible(t *testing.T) {
	contracttest.AssertAccessible(t,
		func(label string) facet.FacetImpl { return NewStatusLight(label) },
		"status",
	)
}
