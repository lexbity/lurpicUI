package action

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/contracttest"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
)

// actionContractArrange drives Measure+Arrange for any action mark through its
// registered layout role. The role is the same object the concrete mark wires
// in its constructor (Layout.OnMeasure/OnArrange), so this matches how the
// package's own tests drive layout.
func actionContractArrange(m facet.FacetImpl, rt facet.RuntimeServices, ctx theme.ResolvedContext, b gfx.Rect) {
	role := m.Base().LayoutRole()
	role.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            ctx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: b.Width(), H: b.Height()}})
	role.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       ctx,
		ParentGroup: role.Parent,
		ChildGroup:  role.Child,
	}, b)
}

func TestActionBar_contract_anchor_export(t *testing.T) {
	_, rt, resolved := newActionBarGoldenFixture(t, defaultActionBarTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	bounds := gfx.RectFromXYWH(0, 0, 720, 1400)

	contracttest.AssertAnchorExport(t,
		func() facet.FacetImpl { return NewActionBar("contract label", nil) },
		func(m facet.FacetImpl, _ facet.AttachContext, b gfx.Rect) {
			actionContractArrange(m, rt, resolved, b)
		},
		bounds, resolved,
	)
}

func TestActionBar_contract_accessible(t *testing.T) {
	contracttest.AssertAccessible(t,
		func(label string) facet.FacetImpl { return NewActionBar(label, nil) },
		"toolbar",
	)
}

func TestActionGroup_contract_anchor_export(t *testing.T) {
	_, rt, resolved := newActionGroupGoldenFixture(t, defaultActionGroupTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	bounds := gfx.RectFromXYWH(0, 0, 720, 1400)

	contracttest.AssertAnchorExport(t,
		func() facet.FacetImpl {
			return NewActionGroup(marks.Const("contract group"), marks.Const([]ActionGroupAction{
				{Key: "a", Label: "Alpha"},
				{Key: "b", Label: "Beta"},
			}))
		},
		func(m facet.FacetImpl, _ facet.AttachContext, b gfx.Rect) {
			actionContractArrange(m, rt, resolved, b)
		},
		bounds, resolved,
	)
}

func TestActionGroup_contract_accessible(t *testing.T) {
	contracttest.AssertAccessible(t,
		func(label string) facet.FacetImpl {
			return NewActionGroup(marks.Const(label), marks.Const([]ActionGroupAction(nil)))
		},
		"group",
	)
}

func TestButton_contract_anchor_export(t *testing.T) {
	_, rt := newTestButton(t, false)
	ctx := theme.DefaultResolvedContext()
	bounds := gfx.RectFromXYWH(0, 0, 720, 1400)

	contracttest.AssertAnchorExport(t,
		func() facet.FacetImpl {
			return NewButton(marks.Const("contract button"), marks.Const(uiinput.ButtonFilled))
		},
		func(m facet.FacetImpl, _ facet.AttachContext, b gfx.Rect) {
			actionContractArrange(m, rt, ctx, b)
		},
		bounds, ctx,
	)
}

func TestButton_contract_accessible(t *testing.T) {
	contracttest.AssertAccessible(t,
		func(label string) facet.FacetImpl {
			return NewButton(marks.Const(label), marks.Const(uiinput.ButtonFilled))
		},
		"button",
	)
}

func TestCommandPalette_contract_anchor_export(t *testing.T) {
	_, rt, resolved := newCommandPaletteFixture(t, theme.DefaultTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	registry := rt.registry
	bounds := gfx.RectFromXYWH(0, 0, 720, 1400)

	contracttest.AssertAnchorExport(t,
		func() facet.FacetImpl {
			return NewCommandPalette(marks.Const("contract palette"), registry, store.NewValueStore(true))
		},
		func(m facet.FacetImpl, _ facet.AttachContext, b gfx.Rect) {
			actionContractArrange(m, rt, resolved, b)
		},
		bounds, resolved,
	)
}

func TestCommandPalette_contract_accessible(t *testing.T) {
	contracttest.AssertAccessible(t,
		func(label string) facet.FacetImpl {
			return NewCommandPalette(marks.Const(label), nil, store.NewValueStore(true))
		},
		"dialog_combobox",
	)
}

func TestIconButton_contract_anchor_export(t *testing.T) {
	rt := iconButtonRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, theme.DefaultTokens(), nil),
	}
	ctx := theme.DefaultResolvedContext()
	bounds := gfx.RectFromXYWH(0, 0, 720, 1400)

	contracttest.AssertAnchorExport(t,
		func() facet.FacetImpl { return NewIconButton(primitive.IconRef("icon")) },
		func(m facet.FacetImpl, _ facet.AttachContext, b gfx.Rect) {
			actionContractArrange(m, rt, ctx, b)
		},
		bounds, ctx,
	)
}

func TestIconButton_contract_accessible(t *testing.T) {
	contracttest.AssertAccessible(t,
		func(label string) facet.FacetImpl {
			btn := NewIconButton(primitive.IconRef("icon"))
			btn.AccessibleLabel = marks.Const(label)
			return btn
		},
		"button",
	)
}

func TestMenuButton_contract_anchor_export(t *testing.T) {
	_, rt := newMenuButtonFixture(t)
	ctx := theme.DefaultResolvedContext()
	bounds := gfx.RectFromXYWH(0, 0, 720, 1400)

	contracttest.AssertAnchorExport(t,
		func() facet.FacetImpl {
			return NewMenuButton("contract menu", []MenuButtonEntry{
				{Key: "a", Label: "Alpha"},
				{Key: "b", Label: "Beta"},
			})
		},
		func(m facet.FacetImpl, _ facet.AttachContext, b gfx.Rect) {
			actionContractArrange(m, rt, ctx, b)
		},
		bounds, ctx,
	)
}

func TestMenuButton_contract_accessible(t *testing.T) {
	contracttest.AssertAccessible(t,
		func(label string) facet.FacetImpl { return NewMenuButton(label, nil) },
		"button_with_menu",
	)
}

func TestPopupPalette_contract_anchor_export(t *testing.T) {
	_, rt, resolved := newPopupPaletteFixture(t, defaultPopupPaletteTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	bounds := gfx.RectFromXYWH(0, 0, 720, 1400)

	contracttest.AssertAnchorExport(t,
		func() facet.FacetImpl {
			return NewPopupPalette("contract palette", []PopupPaletteTool{
				{Key: "brush", Label: "Brush", IconRef: "brush"},
				{Key: "eraser", Label: "Eraser", IconRef: "eraser"},
			}, store.NewValueStore(true))
		},
		func(m facet.FacetImpl, _ facet.AttachContext, b gfx.Rect) {
			actionContractArrange(m, rt, resolved, b)
		},
		bounds, resolved,
	)
}

func TestPopupPalette_contract_accessible(t *testing.T) {
	contracttest.AssertAccessible(t,
		func(label string) facet.FacetImpl {
			return NewPopupPalette(label, nil, store.NewValueStore(true))
		},
		"toolbar",
	)
}

func TestRadialMenu_contract_anchor_export(t *testing.T) {
	_, rt, resolved := newRadialMenuGoldenFixture(t, defaultRadialMenuTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	bounds := gfx.RectFromXYWH(0, 0, 720, 1400)

	contracttest.AssertAnchorExport(t,
		func() facet.FacetImpl { return NewRadialMenu("contract radial", nil, nil) },
		func(m facet.FacetImpl, _ facet.AttachContext, b gfx.Rect) {
			actionContractArrange(m, rt, resolved, b)
		},
		bounds, resolved,
	)
}

func TestRadialMenu_contract_accessible(t *testing.T) {
	contracttest.AssertAccessible(t,
		func(label string) facet.FacetImpl { return NewRadialMenu(label, nil, nil) },
		"radial_menu",
	)
}

func TestRibbon_contract_anchor_export(t *testing.T) {
	_, rt := newRibbonFixture(t, defaultActionBarTokens())
	ctx := theme.DefaultResolvedContext()
	bounds := gfx.RectFromXYWH(0, 0, 720, 1400)

	contracttest.AssertAnchorExport(t,
		func() facet.FacetImpl {
			return NewRibbon("contract ribbon", []RibbonSection{
				{Key: "home", Label: "Home"},
			})
		},
		func(m facet.FacetImpl, _ facet.AttachContext, b gfx.Rect) {
			actionContractArrange(m, rt, ctx, b)
		},
		bounds, ctx,
	)
}

func TestRibbon_contract_accessible(t *testing.T) {
	contracttest.AssertAccessible(t,
		func(label string) facet.FacetImpl { return NewRibbon(label, nil) },
		"toolbar",
	)
}

func TestSplitButton_contract_anchor_export(t *testing.T) {
	_, rt := newSplitButtonFixture(t)
	ctx := theme.DefaultResolvedContext()
	bounds := gfx.RectFromXYWH(0, 0, 720, 1400)

	contracttest.AssertAnchorExport(t,
		func() facet.FacetImpl {
			return NewSplitButton("contract split", []SplitButtonItem{
				{Key: "a", Label: "Alpha"},
				{Key: "b", Label: "Beta"},
			})
		},
		func(m facet.FacetImpl, _ facet.AttachContext, b gfx.Rect) {
			actionContractArrange(m, rt, ctx, b)
		},
		bounds, ctx,
	)
}

func TestSplitButton_contract_accessible(t *testing.T) {
	contracttest.AssertAccessible(t,
		func(label string) facet.FacetImpl { return NewSplitButton(label, nil) },
		"split_button",
	)
}

func TestToolbar_contract_anchor_export(t *testing.T) {
	_, rt := newToolbarFixture(t)
	ctx := theme.DefaultResolvedContext()
	bounds := gfx.RectFromXYWH(0, 0, 720, 1400)

	contracttest.AssertAnchorExport(t,
		func() facet.FacetImpl {
			return NewToolbar(marks.Const("contract toolbar"), []ToolbarGroup{
				{
					Key: "primary",
					Actions: []ActionGroupAction{
						{Key: "a", Label: "Alpha"},
						{Key: "b", Label: "Beta"},
					},
				},
			}, nil)
		},
		func(m facet.FacetImpl, _ facet.AttachContext, b gfx.Rect) {
			actionContractArrange(m, rt, ctx, b)
		},
		bounds, ctx,
	)
}

func TestToolbar_contract_accessible(t *testing.T) {
	contracttest.AssertAccessible(t,
		func(label string) facet.FacetImpl {
			return NewToolbar(marks.Const(label), nil, nil)
		},
		"toolbar",
	)
}

func TestButton_contract_binding_not_severed(t *testing.T) {
	contracttest.AssertBindingNotSevered[string](
		t,
		func() *store.ValueStore[string] { return store.NewValueStore("Save") },
		func(s *store.ValueStore[string]) facet.FacetImpl {
			return NewButton(marks.FromStore(s, facet.DirtyLayout|facet.DirtyProjection), marks.Const(uiinput.ButtonFilled))
		},
		func(m facet.FacetImpl) {
			b := m.(*Button)
			b.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: 1, Y: 1}, Button: platform.PointerLeft})
			b.onPointer(facet.PointerEvent{Kind: platform.PointerRelease, Position: gfx.Point{X: 1, Y: 1}, Button: platform.PointerLeft})
		},
		func(m facet.FacetImpl) string {
			return m.(*Button).Label.Get()
		},
	)
}
