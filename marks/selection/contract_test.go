package selection

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/contracttest"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme"
)

const contractBoundsWidth float32 = 720
const contractBoundsHeight float32 = 1400

func contractBounds() gfx.Rect {
	return gfx.RectFromXYWH(0, 0, contractBoundsWidth, contractBoundsHeight)
}

func contractArrange(m facet.FacetImpl, rt facet.RuntimeServices, ctx theme.ResolvedContext, b gfx.Rect) {
	lr := m.Base().LayoutRole()
	lr.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            ctx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: b.Width(), H: b.Height()}})
	lr.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       ctx,
		ParentGroup: lr.Parent,
		ChildGroup:  lr.Child,
	}, b)
}

func TestButtonGroup_contract_group_children(t *testing.T) {
	contracttest.AssertGroupChildren(t,
		func() facet.FacetImpl {
			return NewButtonGroup("label", []ButtonGroupOption{
				{Key: "a", Label: "A"},
				{Key: "b", Label: "B"},
			}, store.NewValueStore([]string{}))
		},
		func(facet.FacetImpl) {},
	)
}

func TestButtonGroup_contract_anchor_export(t *testing.T) {
	bounds := contractBounds()
	rt := contracttest.NoopRuntime{}
	ctx := theme.DefaultResolvedContext()
	contracttest.AssertAnchorExport(t,
		func() facet.FacetImpl {
			return NewButtonGroup("label", nil, store.NewValueStore([]string{}))
		},
		func(m facet.FacetImpl, _ facet.AttachContext, b gfx.Rect) {
			contractArrange(m, rt, ctx, b)
		},
		bounds,
		ctx,
	)
}

func TestButtonGroup_contract_accessible(t *testing.T) {
	contracttest.AssertAccessible(t,
		func(label string) facet.FacetImpl {
			return NewButtonGroup(label, nil, store.NewValueStore([]string{}))
		},
		"group",
	)
}

func TestCheckbox_contract_anchor_export(t *testing.T) {
	bounds := contractBounds()
	rt := contracttest.NoopRuntime{}
	ctx := theme.DefaultResolvedContext()
	contracttest.AssertAnchorExport(t,
		func() facet.FacetImpl {
			return NewCheckbox("label", store.NewValueStore(CheckboxStateOff))
		},
		func(m facet.FacetImpl, _ facet.AttachContext, b gfx.Rect) {
			contractArrange(m, rt, ctx, b)
		},
		bounds,
		ctx,
	)
}

func TestCheckbox_contract_accessible(t *testing.T) {
	contracttest.AssertAccessible(t,
		func(label string) facet.FacetImpl {
			return NewCheckbox(label, store.NewValueStore(CheckboxStateOff))
		},
		"checkbox",
	)
}

func TestDropdownSelect_contract_group_children(t *testing.T) {
	contracttest.AssertGroupChildren(t,
		func() facet.FacetImpl {
			ds := NewDropdownSelect("label", []DropdownOption{{Value: "a", Label: "A"}}, store.NewValueStore(""))
			ds.open = true
			return ds
		},
		func(facet.FacetImpl) {},
	)
}

func TestDropdownSelect_contract_anchor_export(t *testing.T) {
	bounds := contractBounds()
	rt := contracttest.NoopRuntime{}
	ctx := theme.DefaultResolvedContext()
	contracttest.AssertAnchorExport(t,
		func() facet.FacetImpl {
			return NewDropdownSelect("label", nil, store.NewValueStore(""))
		},
		func(m facet.FacetImpl, _ facet.AttachContext, b gfx.Rect) {
			contractArrange(m, rt, ctx, b)
		},
		bounds,
		ctx,
	)
}

func TestDropdownSelect_contract_accessible(t *testing.T) {
	contracttest.AssertAccessible(t,
		func(label string) facet.FacetImpl {
			return NewDropdownSelect(label, nil, store.NewValueStore(""))
		},
		"combobox",
	)
}

func TestListItem_contract_anchor_export(t *testing.T) {
	bounds := contractBounds()
	rt := contracttest.NoopRuntime{}
	ctx := theme.DefaultResolvedContext()
	contracttest.AssertAnchorExport(t,
		func() facet.FacetImpl {
			return NewListItem(marks.Const("label"))
		},
		func(m facet.FacetImpl, _ facet.AttachContext, b gfx.Rect) {
			contractArrange(m, rt, ctx, b)
		},
		bounds,
		ctx,
	)
}

func TestListItem_contract_accessible(t *testing.T) {
	contracttest.AssertAccessible(t,
		func(label string) facet.FacetImpl {
			return NewListItem(marks.Const(label))
		},
		"option",
	)
}

func TestRadioGroup_contract_anchor_export(t *testing.T) {
	bounds := contractBounds()
	fonts := testkit.TestFontRegistry(t)
	rt := sliderRuntimeStub{fonts: fonts}
	ctx := theme.DefaultResolvedContext().WithFontRegistry(fonts)
	contracttest.AssertAnchorExport(t,
		func() facet.FacetImpl {
			return NewRadioGroup("label", nil, store.NewValueStore(""))
		},
		func(m facet.FacetImpl, _ facet.AttachContext, b gfx.Rect) {
			contractArrange(m, rt, ctx, b)
		},
		bounds,
		ctx,
	)
}

func TestRadioGroup_contract_accessible(t *testing.T) {
	contracttest.AssertAccessible(t,
		func(label string) facet.FacetImpl {
			return NewRadioGroup(label, nil, store.NewValueStore(""))
		},
		"radiogroup",
	)
}

func TestSlider_contract_group_children(t *testing.T) {
	contracttest.AssertGroupChildren(t,
		func() facet.FacetImpl {
			return NewSlider("label", 0, 100, 1, store.NewValueStore(0.0))
		},
		func(facet.FacetImpl) {},
	)
}

func TestSlider_contract_anchor_export(t *testing.T) {
	bounds := contractBounds()
	rt := contracttest.NoopRuntime{}
	ctx := theme.DefaultResolvedContext()
	contracttest.AssertAnchorExport(t,
		func() facet.FacetImpl {
			return NewSlider("label", 0, 100, 1, store.NewValueStore(0.0))
		},
		func(m facet.FacetImpl, _ facet.AttachContext, b gfx.Rect) {
			contractArrange(m, rt, ctx, b)
		},
		bounds,
		ctx,
	)
}

func TestSlider_contract_accessible(t *testing.T) {
	contracttest.AssertAccessible(t,
		func(label string) facet.FacetImpl {
			return NewSlider(label, 0, 100, 1, store.NewValueStore(0.0))
		},
		"slider",
	)
}

func TestSwitch_contract_anchor_export(t *testing.T) {
	bounds := contractBounds()
	rt := contracttest.NoopRuntime{}
	ctx := theme.DefaultResolvedContext()
	contracttest.AssertAnchorExport(t,
		func() facet.FacetImpl {
			return NewSwitch("label", store.NewValueStore(false))
		},
		func(m facet.FacetImpl, _ facet.AttachContext, b gfx.Rect) {
			contractArrange(m, rt, ctx, b)
		},
		bounds,
		ctx,
	)
}

func TestSwitch_contract_accessible(t *testing.T) {
	contracttest.AssertAccessible(t,
		func(label string) facet.FacetImpl {
			return NewSwitch(label, store.NewValueStore(false))
		},
		"switch",
	)
}

func TestTurnDial_contract_accessible(t *testing.T) {
	contracttest.AssertAccessible(t,
		func(label string) facet.FacetImpl {
			return NewTurnDial(label, 0, 100, 1, store.NewValueStore(0.0))
		},
		"slider",
	)
}
