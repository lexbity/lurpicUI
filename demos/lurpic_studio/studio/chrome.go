package studio

import (
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/action"
	"codeburg.org/lexbit/lurpicui/marks/navigation"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
)

type ChromeBar struct {
	facet.Facet
	layout     facet.LayoutRole
	ribbon     *action.Ribbon
	toolbar    *action.Toolbar
	splitBtn   *action.SplitButton
	menuBtn    *action.MenuButton
	breadcrumbs *navigation.Breadcrumbs
	actionBar  *action.ActionBar
	iconBtns   []*action.IconButton

	appState *state.AppState
}

func newChromeRibbon() *action.Ribbon {
	return action.NewRibbon("Main Ribbon", []action.RibbonSection{
		{
			Key: "home", Label: "Home", IconRef: "home",
			Toolbars: []*action.Toolbar{
				action.NewToolbar(marks.Const("File"), []action.ToolbarGroup{
					{
						Key: "file",
						Actions: []action.ActionGroupAction{
							{Key: "new", Label: "New", IconRef: "file_new"},
							{Key: "open", Label: "Open", IconRef: "file_open"},
							{Key: "save", Label: "Save", IconRef: "save"},
						},
					},
					{
						Key: "data",
						Actions: []action.ActionGroupAction{
							{Key: "reload", Label: "Reload", IconRef: "refresh"},
							{Key: "import", Label: "Import", IconRef: "upload"},
						},
					},
				}, nil),
			},
		},
		{
			Key: "insert", Label: "Insert", IconRef: "add",
			Toolbars: []*action.Toolbar{
				action.NewToolbar(marks.Const("Elements"), []action.ToolbarGroup{
					{
						Key: "chart",
						Actions: []action.ActionGroupAction{
							{Key: "add_rule", Label: "Reference Line", IconRef: "horizontal_rule"},
							{Key: "add_annotation", Label: "Annotation", IconRef: "note_add"},
						},
					},
				}, nil),
			},
		},
		{
			Key: "chart", Label: "Chart", IconRef: "bar_chart",
			Toolbars: []*action.Toolbar{
				action.NewToolbar(marks.Const("Type"), []action.ToolbarGroup{
					{
						Key: "type",
						Actions: []action.ActionGroupAction{
							{Key: "line", Label: "Line", IconRef: "show_chart", Active: true},
							{Key: "bar", Label: "Bar", IconRef: "bar_chart"},
							{Key: "area", Label: "Area", IconRef: "area_chart"},
							{Key: "scatter", Label: "Scatter", IconRef: "scatter_plot"},
						},
					},
				}, nil),
			},
		},
		{
			Key: "view", Label: "View", IconRef: "visibility",
			Toolbars: []*action.Toolbar{
				action.NewToolbar(marks.Const("Display"), []action.ToolbarGroup{
					{
						Key: "display",
						Actions: []action.ActionGroupAction{
							{Key: "toggle_grid", Label: "Grid", IconRef: "grid_on"},
							{Key: "toggle_legend", Label: "Legend", IconRef: "legend_toggle"},
						},
					},
				}, nil),
			},
		},
	})
}

func newChromeToolbar() *action.Toolbar {
	return action.NewToolbar(marks.Const("Actions"), []action.ToolbarGroup{
		{
			Key: "edit",
			Actions: []action.ActionGroupAction{
				{Key: "undo", Label: "Undo", IconRef: "undo"},
				{Key: "redo", Label: "Redo", IconRef: "redo"},
			},
		},
		{
			Key: "clipboard",
			Actions: []action.ActionGroupAction{
				{Key: "cut", Label: "Cut", IconRef: "cut"},
				{Key: "copy", Label: "Copy", IconRef: "copy"},
				{Key: "paste", Label: "Paste", IconRef: "paste"},
			},
		},
	}, &action.ToolbarOverflow{
		AccessibleLabel: "More actions",
		TriggerIconRef:  "more_horiz",
		Entries: []action.MenuButtonEntry{
			{Key: "settings", Label: "Settings", IconRef: "settings"},
			{Key: "help", Label: "Help", IconRef: "help"},
			{Key: "about", Label: "About", IconRef: "info"},
		},
	})
}

func newChromeSplitButton() *action.SplitButton {
	return action.NewSplitButton("Export", []action.SplitButtonItem{
		{Key: "export_csv", Label: "Export as CSV", IconRef: "description"},
		{Key: "export_png", Label: "Export as PNG", IconRef: "image"},
		{Key: "export_pdf", Label: "Export as PDF", IconRef: "picture_as_pdf"},
	})
}

func newChromeMenuButton() *action.MenuButton {
	return action.NewMenuButton("More", []action.MenuButtonEntry{
		{Key: "duplicate", Label: "Duplicate", IconRef: "content_copy"},
		{Key: "rename", Label: "Rename", IconRef: "edit"},
		{Key: "delete", Label: "Delete", IconRef: "delete", Destructive: true},
	})
}

func newChromeBreadcrumbs() *navigation.Breadcrumbs {
	return navigation.NewBreadcrumbs("Navigation", []navigation.BreadcrumbItem{
		{Label: "Sources"},
		{Label: "Data"},
		{Label: "View"},
	})
}

func newChromeActionBar() *action.ActionBar {
	return action.NewActionBar("Selection", []action.ActionBarAction{
		{Key: "edit", Label: "Edit", IconRef: "edit"},
		{Key: "duplicate", Label: "Duplicate", IconRef: "content_copy"},
		{Key: "delete", Label: "Delete", IconRef: "delete"},
	})
}

func newChromeIconButtons() []*action.IconButton {
	btns := []struct {
		icon  primitive.IconRef
		label string
	}{
		{"undo", "Undo"},
		{"redo", "Redo"},
		{"search", "Search"},
	}
	out := make([]*action.IconButton, len(btns))
	for i, b := range btns {
		btn := action.NewIconButton(primitive.IconRef(b.icon))
		btn.AccessibleLabel = marks.Const(b.label)
		out[i] = btn
	}
	return out
}

func NewChromeBar(appState *state.AppState, windowSize gfx.Size) *ChromeBar {
	c := &ChromeBar{
		appState: appState,
	}
	c.Facet = facet.NewFacet()

	c.ribbon = newChromeRibbon()
	c.toolbar = newChromeToolbar()
	c.splitBtn = newChromeSplitButton()
	c.menuBtn = newChromeMenuButton()
	c.breadcrumbs = newChromeBreadcrumbs()
	c.actionBar = newChromeActionBar()
	c.iconBtns = newChromeIconButtons()

	c.Facet.AddChild(c.ribbon.Base())
	c.Facet.AddChild(c.toolbar.Base())

	c.layout = facet.LayoutRole{
		OnMeasure: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
			return c.onMeasure(ctx, constraints)
		},
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			c.onArrange(bounds)
		},
		Child: facet.GroupChildContract{
			SupportedPlacement: facet.SupportsFree | facet.SupportsLinear,
			Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
				return facet.IntrinsicSize{
					Min:       gfx.Size{W: constraints.MinSize.W, H: 40},
					Preferred: gfx.Size{W: constraints.MaxSize.W, H: 88},
					Max:       gfx.Size{W: constraints.MaxSize.W, H: 200},
				}
			},
			Stretch: facet.StretchPolicy{
				Width:  facet.StretchAlways,
				Height: facet.StretchNever,
			},
		},
	}
	c.AddRole(&c.layout)
	return c
}

func (c *ChromeBar) Base() *facet.Facet { c.Facet.BindImpl(c); return &c.Facet }
func (c *ChromeBar) OnAttach(ctx facet.AttachContext)   {}
func (c *ChromeBar) OnDetach()                          {}
func (c *ChromeBar) OnActivate()                        {}
func (c *ChromeBar) OnDeactivate()                      {}

func (c *ChromeBar) onMeasure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	width := constraints.MaxSize.W
	if width <= 0 {
		width = 1280
	}

	c.ribbon.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: width, H: 48}})
	ribbonH := c.ribbon.Base().LayoutRole().MeasuredSize.H
	if ribbonH <= 0 {
		ribbonH = 48
	}

	toolbarW := width - 360
	if toolbarW < 120 {
		toolbarW = 120
	}
	c.toolbar.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: toolbarW, H: 40}})
	toolH := c.toolbar.Base().LayoutRole().MeasuredSize.H
	if toolH <= 0 {
		toolH = 40
	}

	c.splitBtn.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: 120, H: 36}})
	c.menuBtn.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: 80, H: 36}})
	c.breadcrumbs.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: 200, H: 36}})
	c.actionBar.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: width, H: 36}})

	for _, ib := range c.iconBtns {
		ib.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: 36, H: 36}})
	}

	totalH := ribbonH + toolH
	if c.appState.SelectedSource.Get() != "" {
		totalH += 36
	}

	return facet.MeasureResult{Size: gfx.Size{W: width, H: totalH}}
}

func (c *ChromeBar) onArrange(bounds gfx.Rect) {
	y := bounds.Min.Y

	ribbonH := c.ribbon.Base().LayoutRole().MeasuredSize.H
	if ribbonH <= 0 {
		ribbonH = 48
	}
	c.ribbon.Base().LayoutRole().OnArrange(facet.ArrangeContext{}, gfx.RectFromXYWH(bounds.Min.X, y, bounds.Width(), ribbonH))
	c.ribbon.Base().LayoutRole().ArrangedBounds = gfx.RectFromXYWH(bounds.Min.X, y, bounds.Width(), ribbonH)
	y += ribbonH

	toolH := c.toolbar.Base().LayoutRole().MeasuredSize.H
	if toolH <= 0 {
		toolH = 40
	}
	toolbarW := bounds.Width() - 120
	if toolbarW < 80 {
		toolbarW = 80
	}
	c.toolbar.Base().LayoutRole().OnArrange(facet.ArrangeContext{}, gfx.RectFromXYWH(bounds.Min.X, y, toolbarW, toolH))
	c.toolbar.Base().LayoutRole().ArrangedBounds = gfx.RectFromXYWH(bounds.Min.X, y, toolbarW, toolH)
}

func (c *ChromeBar) RibbonHeight() float32 {
	return c.ribbon.Base().LayoutRole().MeasuredSize.H
}

func (c *ChromeBar) TotalHeight() float32 {
	h := c.ribbon.Base().LayoutRole().MeasuredSize.H
	if h <= 0 {
		h = 48
	}
	toolH := c.toolbar.Base().LayoutRole().MeasuredSize.H
	if toolH <= 0 {
		toolH = 40
	}
	h += toolH
	if c.appState.SelectedSource.Get() != "" {
		actionH := c.actionBar.Base().LayoutRole().MeasuredSize.H
		if actionH > 0 {
			h += actionH
		} else {
			h += 36
		}
	}
	return h
}
