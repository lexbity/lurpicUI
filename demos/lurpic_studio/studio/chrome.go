package studio

import (
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/action"
	"codeburg.org/lexbit/lurpicui/marks/navigation"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
)

func newChromePane(as *state.AppState) (
	*action.Ribbon,
	*action.Toolbar,
	*navigation.Breadcrumbs,
	*action.ActionBar,
) {
	ribbon := newRibbon()
	toolbar := newToolbar()
	breadcrumbs := newBreadcrumbs(as)
	actionBar := newActionBar(as)
	return ribbon, toolbar, breadcrumbs, actionBar
}

func newRibbon() *action.Ribbon {
	sections := []action.RibbonSection{
		{
			Key:   "home",
			Label: "Home",
			Toolbars: []*action.Toolbar{
				action.NewToolbar(marks.Const("Clipboard"), []action.ToolbarGroup{
					{
						Key: "clipboard",
						Actions: []action.ActionGroupAction{
							{Key: "cut", Label: "Cut", IconRef: "cut"},
							{Key: "copy", Label: "Copy", IconRef: "copy"},
							{Key: "paste", Label: "Paste", IconRef: "paste"},
						},
					},
				}, nil),
				action.NewToolbar(marks.Const("Actions"), []action.ToolbarGroup{
					{
						Key: "actions",
						Actions: []action.ActionGroupAction{
							{Key: "add-source", Label: "Add Source", IconRef: "add"},
							{Key: "refresh", Label: "Refresh", IconRef: "refresh"},
						},
					},
				}, nil),
			},
		},
		{
			Key:   "data",
			Label: "Data",
			Toolbars: []*action.Toolbar{
				action.NewToolbar(marks.Const("Filter"), []action.ToolbarGroup{
					{
						Key: "filter",
						Actions: []action.ActionGroupAction{
							{Key: "sort", Label: "Sort", IconRef: "sort"},
							{Key: "filter", Label: "Filter", IconRef: "filter"},
						},
					},
				}, nil),
				action.NewToolbar(marks.Const("Transform"), []action.ToolbarGroup{
					{
						Key: "transform",
						Actions: []action.ActionGroupAction{
							{Key: "aggregate", Label: "Aggregate", IconRef: "aggregate"},
							{Key: "pivot", Label: "Pivot", IconRef: "pivot"},
						},
					},
				}, nil),
			},
		},
		{
			Key:   "view",
			Label: "View",
			Toolbars: []*action.Toolbar{
				action.NewToolbar(marks.Const("Layout"), []action.ToolbarGroup{
					{
						Key: "layout",
						Actions: []action.ActionGroupAction{
							{Key: "toggle-sources", Label: labelSources, Active: true},
							{Key: "toggle-inspector", Label: "Inspector", Active: true},
						},
					},
				}, nil),
			},
		},
	}
	return action.NewRibbon("App Ribbon", sections)
}

func newToolbar() *action.Toolbar {
	groups := []action.ToolbarGroup{
		{
			Key: "file",
			Actions: []action.ActionGroupAction{
				{Key: "new", IconRef: "new", AccessibleLabel: "New"},
				{Key: "open", IconRef: "open", AccessibleLabel: "Open"},
				{Key: "save", IconRef: "save", AccessibleLabel: "Save"},
			},
		},
		{
			Key: "export",
			Actions: []action.ActionGroupAction{
				{Key: "export-csv", Label: "Export CSV"},
				{Key: "export-pdf", Label: "Export PDF"},
			},
		},
	}
	overflow := &action.ToolbarOverflow{
		AccessibleLabel: "More actions",
		Entries: []action.MenuButtonEntry{
			{Key: "import", Label: "Import Data", IconRef: "import"},
			{Key: "settings", Label: "Settings", IconRef: "settings"},
			{Key: "help", Label: "Help", IconRef: "help"},
			{Label: "", Kind: action.MenuButtonEntryDivider},
			{Key: "about", Label: "About Lurpic Studio"},
		},
	}
	return action.NewToolbar(marks.Const("Main Toolbar"), groups, overflow)
}

func newBreadcrumbs(as *state.AppState) *navigation.Breadcrumbs {
	items := []navigation.BreadcrumbItem{
		{Label: labelSources},
		{Label: "Data"},
	}
	return navigation.NewBreadcrumbs("Path", items)
}

func newActionBar(as *state.AppState) *action.ActionBar {
	return action.NewActionBar("Context", []action.ActionBarAction{
		{Key: "add-source", Label: "Add Source", IconRef: "add"},
		{Key: "remove-source", Label: "Remove", IconRef: "delete"},
	})
}

var _ = primitive.IconRef("")
