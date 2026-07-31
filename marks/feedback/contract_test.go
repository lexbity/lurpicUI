package feedback

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks/contracttest"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme"
)

func TestAlert_contract_group_children(t *testing.T) {
	contracttest.AssertGroupChildren(t,
		func() facet.FacetImpl { return NewAlert("title", "body") },
		func(facet.FacetImpl) {},
	)
}

func TestAlert_contract_anchor_export(t *testing.T) {
	bounds := gfx.RectFromXYWH(0, 0, 720, 1400)
	tokens := alertTokens()
	rt := alertRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, tokens, nil),
		fonts:     testkit.TestFontRegistry(t),
	}
	ctx := alertResolvedContext(tokens, theme.DensityIDComfortable, layout.WritingDirectionLTR)
	contracttest.AssertAnchorExport(t,
		func() facet.FacetImpl { return newAlertFixture() },
		func(m facet.FacetImpl, _ facet.AttachContext, b gfx.Rect) {
			x := m.(*Alert)
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

func TestAlert_contract_accessible(t *testing.T) {
	contracttest.AssertAccessible(t,
		func(label string) facet.FacetImpl { return NewAlert(label, "") },
		"alert",
	)
}

func TestDialog_contract_group_children(t *testing.T) {
	contracttest.AssertGroupChildren(t,
		func() facet.FacetImpl { return NewDialog("title", "body", nil, store.NewValueStore(true)) },
		func(facet.FacetImpl) {},
	)
}

func TestDialog_contract_anchor_export(t *testing.T) {
	bounds := gfx.RectFromXYWH(0, 0, 720, 1400)
	tokens := dialogTokens()
	rt := dialogRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, tokens, nil),
		fonts:     testkit.TestFontRegistry(t),
	}
	ctx := alertResolvedContext(tokens, theme.DensityIDComfortable, layout.WritingDirectionLTR)
	contracttest.AssertAnchorExport(t,
		func() facet.FacetImpl { return newDialogFixture() },
		func(m facet.FacetImpl, _ facet.AttachContext, b gfx.Rect) {
			x := m.(*Dialog)
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

func TestDialog_contract_accessible(t *testing.T) {
	contracttest.AssertAccessible(t,
		func(label string) facet.FacetImpl { return NewDialog(label, "", nil, store.NewValueStore(true)) },
		"dialog",
	)
}

func TestNotification_contract_group_children(t *testing.T) {
	contracttest.AssertGroupChildren(t,
		func() facet.FacetImpl { return NewNotification("title", "body", store.NewValueStore(true)) },
		func(facet.FacetImpl) {},
	)
}

func TestNotification_contract_anchor_export(t *testing.T) {
	bounds := gfx.RectFromXYWH(0, 0, 720, 1400)
	tokens := notificationTokens()
	rt := notificationRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, tokens, nil),
		fonts:     testkit.TestFontRegistry(t),
	}
	ctx := notificationResolvedContext(tokens, theme.DensityIDComfortable, layout.WritingDirectionLTR)
	contracttest.AssertAnchorExport(t,
		func() facet.FacetImpl { return newNotificationFixture() },
		func(m facet.FacetImpl, _ facet.AttachContext, b gfx.Rect) {
			x := m.(*Notification)
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

func TestNotification_contract_accessible(t *testing.T) {
	contracttest.AssertAccessible(t,
		func(label string) facet.FacetImpl { return NewNotification(label, "", store.NewValueStore(true)) },
		"status",
	)
}

func TestTooltip_contract_group_children(t *testing.T) {
	contracttest.AssertGroupChildren(t,
		func() facet.FacetImpl { return NewTooltip("content", store.NewValueStore(true)) },
		func(facet.FacetImpl) {},
	)
}

func TestTooltip_contract_anchor_export(t *testing.T) {
	bounds := gfx.RectFromXYWH(0, 0, 720, 1400)
	tokens := tooltipTokens()
	rt := alertRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, tokens, nil),
		fonts:     testkit.TestFontRegistry(t),
	}
	ctx := alertResolvedContext(tokens, theme.DensityIDComfortable, layout.WritingDirectionLTR)
	contracttest.AssertAnchorExport(t,
		func() facet.FacetImpl { return newTooltipFixture() },
		func(m facet.FacetImpl, _ facet.AttachContext, b gfx.Rect) {
			x := m.(*Tooltip)
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

func TestTooltip_contract_accessible(t *testing.T) {
	contracttest.AssertAccessible(t,
		func(label string) facet.FacetImpl { return NewTooltip(label, store.NewValueStore(true)) },
		"tooltip",
	)
}
