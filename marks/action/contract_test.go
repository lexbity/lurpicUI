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
	scope := store.NewScope()
	t.Cleanup(scope.Close)
	_, rt, resolved := newCommandPaletteFixture(t, theme.DefaultTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	registry := rt.registry
	bounds := gfx.RectFromXYWH(0, 0, 720, 1400)

	contracttest.AssertAnchorExport(t,
		func() facet.FacetImpl {
			return NewCommandPalette(scope, marks.Const("contract palette"), registry, store.NewValueStore(scope, true))
		},
		func(m facet.FacetImpl, _ facet.AttachContext, b gfx.Rect) {
			actionContractArrange(m, rt, resolved, b)
		},
		bounds, resolved,
	)
}

func TestCommandPalette_contract_accessible(t *testing.T) {
	scope := store.NewScope()
	t.Cleanup(scope.Close)
	contracttest.AssertAccessible(t,
		func(label string) facet.FacetImpl {
			return NewCommandPalette(scope, marks.Const(label), nil, store.NewValueStore(scope, true))
		},
		"dialog_combobox",
	)
}

func TestIconButton_contract_anchor_export(t *testing.T) {
	scope := store.NewScope()
	t.Cleanup(scope.Close)
	rt := iconButtonRuntimeStub{
		rootStyle: theme.NewRootStyleContext(scope, nil, theme.DefaultTokens(), nil),
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

func TestRibbon_contract_anchor_export(t *testing.T) {
	scope := store.NewScope()
	t.Cleanup(scope.Close)
	_, rt := newRibbonFixture(t, defaultActionBarTokens())
	ctx := theme.DefaultResolvedContext()
	bounds := gfx.RectFromXYWH(0, 0, 720, 1400)

	contracttest.AssertAnchorExport(t,
		func() facet.FacetImpl {
			return NewRibbon(scope, "contract ribbon", []RibbonSection{
				{Key: "home", Label: "Home"},
			}, nil)
		},
		func(m facet.FacetImpl, _ facet.AttachContext, b gfx.Rect) {
			actionContractArrange(m, rt, ctx, b)
		},
		bounds, ctx,
	)
}

func TestRibbon_contract_accessible(t *testing.T) {
	scope := store.NewScope()
	t.Cleanup(scope.Close)
	contracttest.AssertAccessible(t,
		func(label string) facet.FacetImpl { return NewRibbon(scope, label, nil, nil) },
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
	scope := store.NewScope()
	t.Cleanup(scope.Close)
	contracttest.AssertBindingNotSevered[string](
		t,
		func() *store.ValueStore[string] { return store.NewValueStore(scope, "Save") },
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

// The Focusable contract tests below are the LL033 proofs for every action
// mark that declares Focusable() directly (the value-contract work added the
// direct declarations; the static rule requires the dynamic helper to be
// wired in the same package).

func TestActionBar_contract_focusable(t *testing.T) {
	contracttest.AssertFocusable(t, func(disabled bool) facet.FacetImpl {
		bar := NewActionBar("actions", nil)
		bar.Disabled = marks.Const(disabled)
		return bar
	})
}

func TestActionGroup_contract_focusable(t *testing.T) {
	contracttest.AssertFocusable(t, func(disabled bool) facet.FacetImpl {
		group := NewActionGroup(marks.Const("group"), marks.Const([]ActionGroupAction{{Key: "k", Label: "L"}}))
		group.Disabled = marks.Const(disabled)
		return group
	})
}

func TestButton_contract_focusable(t *testing.T) {
	contracttest.AssertFocusable(t, func(disabled bool) facet.FacetImpl {
		button := NewButton(marks.Const("save"), marks.Const(uiinput.ButtonFilled))
		button.Disabled = marks.Const(disabled)
		return button
	})
}

func TestCommandPalette_contract_focusable(t *testing.T) {
	scope := store.NewScope()
	t.Cleanup(scope.Close)
	contracttest.AssertFocusable(t, func(disabled bool) facet.FacetImpl {
		// The palette's focus gate is Disabled && Open; the contract builder
		// proves the enabled path, so the palette is constructed open.
		palette := NewCommandPalette(scope, marks.Const("palette"), nil, store.NewValueStore(scope, true))
		palette.Disabled = marks.Const(disabled)
		return palette
	})
}

func TestIconButton_contract_focusable(t *testing.T) {
	contracttest.AssertFocusable(t, func(disabled bool) facet.FacetImpl {
		button := NewIconButton(primitive.IconSVG("M0 0h1v1z"))
		button.Disabled = marks.Const(disabled)
		return button
	})
}

func TestMenuButton_contract_focusable(t *testing.T) {
	contracttest.AssertFocusable(t, func(disabled bool) facet.FacetImpl {
		menu := NewMenuButton("menu", nil)
		menu.Disabled = marks.Const(disabled)
		return menu
	})
}

func TestRibbon_contract_focusable(t *testing.T) {
	scope := store.NewScope()
	t.Cleanup(scope.Close)
	contracttest.AssertFocusable(t, func(disabled bool) facet.FacetImpl {
		ribbon := NewRibbon(scope, "ribbon", []RibbonSection{{Key: "home", Label: "Home"}}, nil)
		ribbon.Disabled = marks.Const(disabled)
		return ribbon
	})
}

func TestSplitButton_contract_focusable(t *testing.T) {
	contracttest.AssertFocusable(t, func(disabled bool) facet.FacetImpl {
		split := NewSplitButton("split", nil)
		split.Disabled = marks.Const(disabled)
		return split
	})
}

func TestToolbar_contract_focusable(t *testing.T) {
	contracttest.AssertFocusable(t, func(disabled bool) facet.FacetImpl {
		toolbar := NewToolbar(marks.Const("toolbar"), []ToolbarGroup{{Key: "g", Actions: []ActionGroupAction{{Key: "k", Label: "L"}}}}, nil)
		toolbar.Disabled = marks.Const(disabled)
		return toolbar
	})
}
