package primitive

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/contracttest"
	"codeburg.org/lexbit/lurpicui/theme"
)

func TestIcon_contract_anchor_export(t *testing.T) {
	rt := contracttest.NoopRuntime{}
	ctx := theme.DefaultResolvedContext()
	bounds := gfx.RectFromXYWH(0, 0, 720, 1400)

	contracttest.AssertAnchorExport(t,
		func() facet.FacetImpl { return NewIcon(IconRef("home")) },
		func(m facet.FacetImpl, _ facet.AttachContext, b gfx.Rect) {
			i := m.(*Icon)
			i.Layout.Measure(facet.MeasureContext{
				Runtime:          rt,
				Theme:            ctx,
				ContentScale:     1,
				Density:          facet.DensityID(theme.DensityIDComfortable),
				WritingDirection: facet.WritingDirectionLTR,
			}, facet.Constraints{MaxSize: gfx.Size{W: b.Width(), H: b.Height()}})
			i.Layout.Arrange(facet.ArrangeContext{
				Runtime:     rt,
				Theme:       ctx,
				ParentGroup: i.Layout.Parent,
				ChildGroup:  i.Layout.Child,
			}, b)
		},
		bounds,
		ctx,
	)
}

func TestIcon_contract_accessible(t *testing.T) {
	contracttest.AssertAccessible(t,
		func(label string) facet.FacetImpl {
			icon := NewIcon(IconRef("home"))
			icon.Decorative = marks.Const(false)
			icon.AccessibleLabel = marks.Const(label)
			return icon
		},
		"img",
	)
}

func TestText_contract_anchor_export(t *testing.T) {
	rt := textRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, theme.DefaultTokens(), nil),
		fonts:     testkit.TestFontRegistry(t),
	}
	ctx := theme.DefaultResolvedContext()
	bounds := gfx.RectFromXYWH(0, 0, 720, 1400)

	contracttest.AssertAnchorExport(t,
		func() facet.FacetImpl { return NewText(marks.Const("Hello world")) },
		func(m facet.FacetImpl, _ facet.AttachContext, b gfx.Rect) {
			txt := m.(*Text)
			txt.Layout.Measure(facet.MeasureContext{
				Runtime:          rt,
				Theme:            ctx,
				ContentScale:     1,
				Density:          facet.DensityID(theme.DensityIDComfortable),
				WritingDirection: facet.WritingDirectionLTR,
			}, facet.Constraints{MaxSize: gfx.Size{W: b.Width(), H: b.Height()}})
			txt.Layout.Arrange(facet.ArrangeContext{
				Runtime:     rt,
				Theme:       ctx,
				ParentGroup: txt.Layout.Parent,
				ChildGroup:  txt.Layout.Child,
			}, b)
		},
		bounds,
		ctx,
	)
}
