package input

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/marks/contracttest"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
)

func TestColorPicker_contract_anchor_export(t *testing.T) {
	rt := contracttest.NoopRuntime{}
	ctx := theme.DefaultResolvedContext()
	bounds := gfx.RectFromXYWH(0, 0, 720, 1400)

	contracttest.AssertAnchorExport(t,
		func() facet.FacetImpl {
			return NewColorPicker("Palette", store.NewValueStore(gfx.Color{R: 1, G: 0, B: 0, A: 1}))
		},
		func(m facet.FacetImpl, _ facet.AttachContext, b gfx.Rect) {
			c := m.(*ColorPicker)
			c.Layout.Measure(facet.MeasureContext{
				Runtime:          rt,
				Theme:            ctx,
				ContentScale:     1,
				Density:          facet.DensityID(theme.DensityIDComfortable),
				WritingDirection: facet.WritingDirectionLTR,
			}, facet.Constraints{MaxSize: gfx.Size{W: b.Width(), H: b.Height()}})
			c.Layout.Arrange(facet.ArrangeContext{
				Runtime:     rt,
				Theme:       ctx,
				ParentGroup: c.Layout.Parent,
				ChildGroup:  c.Layout.Child,
			}, b)
		},
		bounds,
		ctx,
	)
}

func TestColorPicker_contract_accessible(t *testing.T) {
	contracttest.AssertAccessible(t,
		func(label string) facet.FacetImpl {
			return NewColorPicker(label, store.NewValueStore(gfx.Color{R: 1, G: 0, B: 0, A: 1}))
		},
		"colorpicker",
	)
}

func TestNumberField_contract_anchor_export(t *testing.T) {
	rt := numberFieldRuntimeStub{fonts: testkit.TestFontRegistry(t)}
	ctx := theme.DefaultResolvedContext()
	bounds := gfx.RectFromXYWH(0, 0, 720, 1400)

	contracttest.AssertAnchorExport(t,
		func() facet.FacetImpl {
			return NewNumberField("Amount", store.NewValueStore(float64(0)))
		},
		func(m facet.FacetImpl, _ facet.AttachContext, b gfx.Rect) {
			nf := m.(*NumberField)
			nf.Layout.Measure(facet.MeasureContext{
				Runtime:          rt,
				Theme:            ctx,
				ContentScale:     1,
				Density:          facet.DensityID(theme.DensityIDComfortable),
				WritingDirection: facet.WritingDirectionLTR,
			}, facet.Constraints{MaxSize: gfx.Size{W: b.Width(), H: b.Height()}})
			nf.Layout.Arrange(facet.ArrangeContext{
				Runtime:     rt,
				Theme:       ctx,
				ParentGroup: nf.Layout.Parent,
				ChildGroup:  nf.Layout.Child,
			}, b)
		},
		bounds,
		ctx,
	)
}

func TestNumberField_contract_accessible(t *testing.T) {
	contracttest.AssertAccessible(t,
		func(label string) facet.FacetImpl {
			return NewNumberField(label, store.NewValueStore(float64(0)))
		},
		"spinbutton",
	)
}

func TestTextField_contract_anchor_export(t *testing.T) {
	rt := textFieldRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, theme.DefaultTokens(), nil),
		fonts:     testkit.TestFontRegistry(t),
	}
	ctx := theme.DefaultResolvedContext()
	bounds := gfx.RectFromXYWH(0, 0, 720, 1400)

	contracttest.AssertAnchorExport(t,
		func() facet.FacetImpl {
			return NewTextField("Email", uiinput.TextInputOutlined, store.NewValueStore(""))
		},
		func(m facet.FacetImpl, _ facet.AttachContext, b gfx.Rect) {
			tf := m.(*TextField)
			tf.Layout.Measure(facet.MeasureContext{
				Runtime:          rt,
				Theme:            ctx,
				ContentScale:     1,
				Density:          facet.DensityID(theme.DensityIDComfortable),
				WritingDirection: facet.WritingDirectionLTR,
			}, facet.Constraints{MaxSize: gfx.Size{W: b.Width(), H: b.Height()}})
			tf.Layout.Arrange(facet.ArrangeContext{
				Runtime:     rt,
				Theme:       ctx,
				ParentGroup: tf.Layout.Parent,
				ChildGroup:  tf.Layout.Child,
			}, b)
		},
		bounds,
		ctx,
	)
}

func TestTextField_contract_accessible(t *testing.T) {
	contracttest.AssertAccessible(t,
		func(label string) facet.FacetImpl {
			return NewTextField(label, uiinput.TextInputOutlined, store.NewValueStore(""))
		},
		"textbox",
	)
}
