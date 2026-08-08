package studio

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/action"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
)

// playActionFamily is the Action playground: action_bar, action_group, ribbon,
// a standalone toolbar, split_button, menu_button, popup_palette, and
// radial_menu. Each mark's Activated event writes the shared lastAction /
// ribbonSection stores — the visible feedback of a running action command (the
// action family's distinctive behavior: command dispatch).
type playActionFamily struct {
	scroll      *demoList
	bar         *action.ActionBar
	group       *action.ActionGroup
	ribbon      *action.Ribbon
	toolbar     *action.Toolbar
	split       *action.SplitButton
	menu        *action.MenuButton
	palette     *action.PopupPalette
	radial      *action.RadialMenu
	lastAction  *store.ValueStore[string]
	ribbonTab   *store.ValueStore[int]
	paletteOpen *store.ValueStore[bool]
}

// newPlayActionFamily builds the Action family playground.
func newPlayActionFamily() *playActionFamily {
	f := &playActionFamily{
		lastAction:  store.NewValueStore(""),
		ribbonTab:   store.NewValueStore(0),
		paletteOpen: store.NewValueStore(false),
	}

	f.bar = action.NewActionBar("File", []action.ActionBarAction{
		{Key: "new", Label: "New", Variant: uiinput.ButtonFilled},
		{Key: "open", Label: "Open", Variant: uiinput.ButtonOutlined},
		{Key: "save", Label: "Save", Variant: uiinput.ButtonText},
	})

	f.group = action.NewActionGroup(marks.Const("Align"), marks.Const([]action.ActionGroupAction{
		{Key: "left", Label: "L"},
		{Key: "center", Label: "C"},
		{Key: "right", Label: "R"},
	}))

	boldTB := action.NewToolbar(marks.Const("Font"), []action.ToolbarGroup{
		{Key: "font", Actions: []action.ActionGroupAction{
			{Key: "bold", Label: "B"},
			{Key: "italic", Label: "I"},
		}},
	}, nil)
	insertTB := action.NewToolbar(marks.Const("Insert"), []action.ToolbarGroup{
		{Key: "insert", Actions: []action.ActionGroupAction{
			{Key: "image", Label: "Image"},
			{Key: "link", Label: "Link"},
		}},
	}, nil)
	f.ribbon = action.NewRibbon("Editing", []action.RibbonSection{
		{Key: "home", Label: "Home", Toolbars: []*action.Toolbar{boldTB}},
		{Key: "insert", Label: "Insert", Toolbars: []*action.Toolbar{insertTB}},
	})

	// A standalone toolbar (the ribbon's toolbars are internal sub-marks, so a
	// hosted toolbar is the tree-reachable instance).
	f.toolbar = action.NewToolbar(marks.Const("Format"), []action.ToolbarGroup{
		{Key: "format", Actions: []action.ActionGroupAction{
			{Key: "bold", Label: "Bold"},
			{Key: "italic", Label: "Italic"},
			{Key: "underline", Label: "Underline"},
		}},
	}, nil)

	f.split = action.NewSplitButton("Export", []action.SplitButtonItem{
		{Key: "export.png", Label: "Export PNG"},
		{Key: "export.csv", Label: "Export CSV"},
	})
	f.menu = action.NewMenuButton("File", []action.MenuButtonEntry{
		{Key: "file.new", Label: "New", Kind: action.MenuButtonEntryItem},
		{Key: "file.open", Label: "Open", Kind: action.MenuButtonEntryItem},
		{Kind: action.MenuButtonEntryDivider},
		{Key: "file.close", Label: "Close", Kind: action.MenuButtonEntryItem, Destructive: true},
	})
	f.palette = action.NewPopupPalette("Brush", []action.PopupPaletteTool{
		{Key: "red", Label: "Red", Color: gfx.ColorFromRGBA8(230, 80, 80, 255)},
		{Key: "green", Label: "Green", Color: gfx.ColorFromRGBA8(80, 200, 115, 255)},
		{Key: "blue", Label: "Blue", Color: gfx.ColorFromRGBA8(80, 140, 245, 255)},
	}, f.paletteOpen)

	// A radial menu: a center glyph ringed by chart-type choices.
	center := primitive.NewText(marks.Const("◈"))
	f.radial = action.NewRadialMenu("Chart", center, []action.RadialChild{
		{Child: primitive.NewText(marks.Const("L")), Placement: facet.RadialPlacement{Angle: 0, RadiusTrack: 90}},
		{Child: primitive.NewText(marks.Const("A")), Placement: facet.RadialPlacement{Angle: 1.5708, RadiusTrack: 90}},
		{Child: primitive.NewText(marks.Const("P")), Placement: facet.RadialPlacement{Angle: 3.1416, RadiusTrack: 90}},
		{Child: primitive.NewText(marks.Const("B")), Placement: facet.RadialPlacement{Angle: 4.7124, RadiusTrack: 90}},
	})

	f.scroll = newDemoList(listGap,
		playgroundCard("action_bar — click an action", f.bar),
		playgroundCard("action_group — click an alignment", f.group),
		playgroundCard("ribbon — click a tab", f.ribbon),
		playgroundCard("toolbar — click a format", f.toolbar),
		playgroundCard("split_button — split an action", f.split),
		playgroundCard("menu_button — open the menu", f.menu),
		playgroundCard("popup_palette — pick a color", f.palette),
		playgroundCard("radial_menu — hover a ring", f.radial),
	)
	return f
}

// wire subscribes the family's Activated dispatches. Subscribers receive on
// the runtime thread (the input pipeline runs there), so writing the store is
// the same thread as the store's own events.
func (f *playActionFamily) wire() func() {
	barID := f.bar.Activated.Subscribe(func(ma action.MarkAction) { f.lastAction.Set(ma.Key) })
	groupID := f.group.Activated.Subscribe(func(ma action.MarkAction) { f.lastAction.Set(ma.Key) })
	ribbonID := f.ribbon.Activated.Subscribe(func(index int) { f.ribbonTab.Set(index) })
	toolbarID := f.toolbar.Activated.Subscribe(func(ma action.MarkAction) { f.lastAction.Set(ma.Key) })
	splitID := f.split.Activated.Subscribe(func(ma action.MarkAction) { f.lastAction.Set(ma.Key) })
	menuID := f.menu.Activated.Subscribe(func(ma action.MarkAction) { f.lastAction.Set(ma.Key) })
	paletteID := f.palette.Activated.Subscribe(func(ma action.MarkAction) { f.lastAction.Set(ma.Key) })
	return func() {
		f.bar.Activated.Unsubscribe(barID)
		f.group.Activated.Unsubscribe(groupID)
		f.ribbon.Activated.Unsubscribe(ribbonID)
		f.toolbar.Activated.Unsubscribe(toolbarID)
		f.split.Activated.Unsubscribe(splitID)
		f.menu.Activated.Unsubscribe(menuID)
		f.palette.Activated.Unsubscribe(paletteID)
	}
}
