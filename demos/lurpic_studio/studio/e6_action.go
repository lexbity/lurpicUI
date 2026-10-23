package studio

import (
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/action"
	"codeburg.org/lexbit/lurpicui/marks/structure"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
)

// playActionFamily is the Action playground: action_bar, action_group, ribbon,
// a standalone toolbar, split_button, and menu_button. Each mark's Activated
// event writes the shared lastAction / ribbonSection stores — the visible
// feedback of a running action command (the action family's distinctive
// behavior: command dispatch).
type playActionFamily struct {
	scroll     *structure.ScrollRegion
	bar        *action.ActionBar
	group      *action.ActionGroup
	ribbon     *action.Ribbon
	toolbar    *action.Toolbar
	split      *action.SplitButton
	menu       *action.MenuButton
	lastAction *store.ValueStore[string]
	ribbonTab  *store.ValueStore[int]
}

// newPlayActionFamily builds the Action family playground.
func newPlayActionFamily() *playActionFamily {
	f := &playActionFamily{
		lastAction: store.NewValueStore(""),
		ribbonTab:  store.NewValueStore(0),
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
	// The ribbon's active section is the app-domain ribbonTab store, injected
	// so the shell reads it directly (RX-2 Q2 injected-truth path) — no
	// mirror subscription.
	f.ribbon = action.NewRibbon("Editing", []action.RibbonSection{
		{Key: "home", Label: "Home", Toolbars: []*action.Toolbar{boldTB}},
		{Key: "insert", Label: "Insert", Toolbars: []*action.Toolbar{insertTB}},
	}, f.ribbonTab)

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
	f.scroll = newPlayScroll(listGap,
		playgroundCard("action_bar — click an action", f.bar),
		playgroundCard("action_group — click an alignment", f.group),
		playgroundCard("ribbon — click a tab", f.ribbon),
		playgroundCard("toolbar — click a format", f.toolbar),
		playgroundCard("split_button — split an action", f.split),
		playgroundCard("menu_button — open the menu", f.menu),
	)
	return f
}

// wire subscribes the family's Activated dispatches. Subscribers receive on
// the runtime thread (the input pipeline runs there), so writing the store is
// the same thread as the store's own events.
func (f *playActionFamily) wire() func() {
	barID := f.bar.Activated.Subscribe(func(ma action.MarkAction) { f.lastAction.Set(ma.Key) })
	groupID := f.group.Activated.Subscribe(func(ma action.MarkAction) { f.lastAction.Set(ma.Key) })
	// The ribbon publishes its selection into the injected ribbonTab store
	// (FR-8 truth); this subscription surfaces the chosen section in the
	// family's lastAction readout.
	ribbonID := f.ribbon.Selected.Subscribe(func(index int) {
		if sections := f.ribbon.Sections.Get(); index >= 0 && index < len(sections) {
			f.lastAction.Set(sections[index].Label)
		}
	})
	toolbarID := f.toolbar.Activated.Subscribe(func(ma action.MarkAction) { f.lastAction.Set(ma.Key) })
	splitID := f.split.Activated.Subscribe(func(ma action.MarkAction) { f.lastAction.Set(ma.Key) })
	menuID := f.menu.Activated.Subscribe(func(ma action.MarkAction) { f.lastAction.Set(ma.Key) })
	return func() {
		f.bar.Activated.Unsubscribe(barID)
		f.group.Activated.Unsubscribe(groupID)
		f.ribbon.Selected.Unsubscribe(ribbonID)
		f.toolbar.Activated.Unsubscribe(toolbarID)
		f.split.Activated.Unsubscribe(splitID)
		f.menu.Activated.Unsubscribe(menuID)
	}
}
