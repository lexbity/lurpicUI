package studio

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/feedback"
	"codeburg.org/lexbit/lurpicui/marks/navigation"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
)

// NarrowShell is the narrow-mode overlay sub-tree (FR-resp): the exhibit index
// re-hosts as a nav_drawer (left edge) plus a bottom action bar of exhibit
// icons, and the inspector re-hosts as a bottom sheet. All three bind the same
// ShellState stores as the wide index/inspector panes, so a breakpoint crossing
// preserves state (F-resp: store identity, never mark pointers).
//
// The bottom bar is a horizontal nav_rail (RX-2 P2 added Orientation: the
// F-rail-shape limitation is retired) whose destinations bind the same
// ActiveExhibit store as the drawer. The scrim is the standard feedback.scrim
// mark (RX-2 FR-5c) instead of a bespoke facet.
type NarrowShell struct {
	facet.Facet
	layout facet.LayoutRole

	shell    *ShellState
	scrim    *feedback.Scrim
	drawer   *navigation.NavDrawer
	bar      *navigation.NavRail
	sheet    *ExhibitInspector
	drawerID *store.ValueStore[int]

	cleanup func()
}

// NewNarrowShell builds the narrow overlay sub-tree over the shared shell
// state. counts is the per-exhibit demonstrated-mark count map (shared with the
// wide inspector so both sheets agree).
func NewNarrowShell(shell *ShellState, counts map[ExhibitID]int) *NarrowShell {
	n := &NarrowShell{
		shell:    shell,
		drawerID: store.NewValueStore(exhibitIndex(shell.ActiveExhibit.Get())),
	}
	n.Facet = facet.NewFacet()

	// The hit-blocking scrim behind the drawer and bottom sheet (FR-17b):
	// it dims the stage and a tap on it (outside the open overlay) dismisses.
	n.scrim = feedback.NewScrim()

	// The nav_drawer re-hosts the exhibit index: sections by concept group,
	// items bound to the same ActiveExhibit store.
	sections := make([]navigation.NavDrawerSection, 0)
	for _, group := range indexGroupOrder() {
		items := make([]navigation.NavDrawerItem, 0)
		for _, e := range exhibitCatalog {
			if e.group == group {
				items = append(items, navigation.NavDrawerItem{Key: string(e.id), Label: e.title, IconRef: e.icon})
			}
		}
		sections = append(sections, navigation.NavDrawerSection{Label: group, Items: items})
	}
	n.drawer = navigation.NewNavDrawer("Exhibits", sections, shell.IndexOpen, n.drawerID)

	// The bottom action bar is the narrow-mode exhibit selector: a
	// horizontal nav_rail over the exhibit destinations, collapsed to icons
	// (the mobile bottom-action-bar pattern; RX-2 P2).
	items := make([]navigation.NavRailItem, 0, len(exhibitCatalog))
	for _, e := range exhibitCatalog {
		items = append(items, navigation.NavRailItem{Key: string(e.id), Label: e.title, IconRef: e.icon})
	}
	n.bar = navigation.NewNavRail("Exhibits", items, n.drawerID)
	n.bar.Orientation = navigation.NavRailHorizontal
	n.bar.Collapsed = marks.Const(true)

	// The bottom sheet re-hosts the inspector (sheet mode: drag handle +
	// Escape dismissal, FR-17c).
	n.sheet = NewSheetInspector(shell, counts)

	n.AddChild(n.scrim.Base())
	n.AddChild(n.drawer.Base())
	n.AddChild(n.bar.Base())
	n.AddChild(n.sheet.Base())

	n.layout = facet.LayoutRole{ //lurpiclint:ignore * -- bespoke narrow-shell host (F-lint-hosts)
		OnMeasure: func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
			return n.measure(ctx, c)
		},
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			n.arrange(ctx, bounds)
		},
	}
	n.layout.Child = linearChildContract(facet.StretchPolicy{
		Width:  facet.StretchAlways,
		Height: facet.StretchAlways,
	})
	n.AddRole(&n.layout)
	return n
}

// indexGroupOrder returns the concept-group labels in catalog order.
func indexGroupOrder() []string {
	out := make([]string, 0)
	for _, e := range exhibitCatalog {
		found := false
		for _, g := range out {
			if g == e.group {
				found = true
				break
			}
		}
		if !found {
			out = append(out, e.group)
		}
	}
	return out
}

func (n *NarrowShell) measure(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
	for _, child := range []facet.FacetImpl{n.drawer, n.bar, n.sheet} {
		if role := child.Base().LayoutRole(); role != nil {
			role.Measure(ctx, facet.Constraints{MaxSize: c.MaxSize})
		}
	}
	return facet.MeasureResult{Size: c.Constrain(c.MaxSize)}
}

// arrange places the narrow overlays. In wide mode the Root arranges this
// sub-tree to zero bounds (nothing shows); in narrow mode the Root arranges it
// over the full stage: the nav_drawer hangs off the left edge when open, the
// bottom action bar sits above the status bar, and the inspector sheet slides
// over the bottom when open.
func (n *NarrowShell) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	// F-layout-root-fallback: the runtime's runLayoutPass can independently
	// re-arrange this sub-tree with the full window bounds when a store change
	// marks it DirtyLayout (its own ArrangedBounds was empty at that instant),
	// overriding Root's wide-mode empty cascade. Consult the shared mode flag
	// and short-circuit to empty children whenever the shell is wide, no matter
	// what bounds the runtime supplied.
	if n.shell.Mode == LayoutWide || bounds.IsEmpty() {
		for _, child := range []facet.FacetImpl{n.scrim, n.drawer, n.bar, n.sheet} {
			if role := child.Base().LayoutRole(); role != nil {
				role.Arrange(ctx, gfx.Rect{})
			}
		}
		return
	}
	barH := n.bar.Base().LayoutRole().MeasuredSize.H
	if barH < 1 {
		barH = 48
	}
	// Bottom action bar: full width, sitting just above the status bar.
	n.bar.Base().LayoutRole().Arrange(ctx, gfx.RectFromXYWH(bounds.Min.X, bounds.Max.Y-barH, bounds.Width(), barH))

	content := gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, bounds.Width(), bounds.Max.Y-barH-bounds.Min.Y)

	// The scrim covers the content area (above the bar), behind the drawer and
	// sheet (FR-17b): it dims the stage and blocks clicks to it. Its arranged
	// bounds are empty while nothing is open, so it contributes no hit region.
	if n.shell.IndexOpen.Get() || n.shell.InspectorOpen.Get() {
		n.scrim.Base().LayoutRole().Arrange(ctx, content)
	} else {
		n.scrim.Base().LayoutRole().Arrange(ctx, gfx.Rect{})
	}

	// Nav drawer: left edge, only when open.
	if n.shell.IndexOpen.Get() {
		drawerW := n.drawer.Base().LayoutRole().MeasuredSize.W
		if drawerW < 1 {
			drawerW = 280
		}
		if drawerW > 320 {
			drawerW = 320
		}
		if drawerW > content.Width()*0.8 {
			drawerW = content.Width() * 0.8
		}
		n.drawer.Base().LayoutRole().Arrange(ctx, gfx.RectFromXYWH(content.Min.X, content.Min.Y, drawerW, content.Height()))
	} else {
		n.drawer.Base().LayoutRole().Arrange(ctx, gfx.Rect{})
	}

	// Inspector bottom sheet: bottom edge, only when open.
	if n.shell.InspectorOpen.Get() {
		sheetH := n.sheet.Base().LayoutRole().MeasuredSize.H
		if sheetH < 1 {
			sheetH = 180
		}
		if sheetH > content.Height()*0.6 {
			sheetH = content.Height() * 0.6
		}
		n.sheet.Base().LayoutRole().Arrange(ctx, gfx.RectFromXYWH(content.Min.X, content.Max.Y-sheetH, content.Width(), sheetH))
	} else {
		n.sheet.Base().LayoutRole().Arrange(ctx, gfx.Rect{})
	}
}

func (n *NarrowShell) OnAttach(ctx facet.AttachContext) {
	// A press on the scrim (outside the drawer/sheet) dismisses both narrow
	// overlays (FR-17b).
	n.scrim.Dismissed.Subscribe(func(signal.Unit) {
		n.shell.IndexOpen.Set(false)
		n.shell.InspectorOpen.Set(false)
	})
	// A bottom-bar destination switches the exhibit (the rail publishes the
	// selection to the shared drawer index store via its FR-8 binding).
	n.bar.Activated.Subscribe(func(index int) {
		if index >= 0 && index < len(exhibitCatalog) {
			n.setActive(exhibitCatalog[index].id)
		}
	})
	drawerID := n.drawer.Activated.Subscribe(func(index int) {
		if index >= 0 && index < len(exhibitCatalog) {
			n.setActive(exhibitCatalog[index].id)
			n.shell.IndexOpen.Set(false)
		}
	})
	activeID := n.shell.ActiveExhibit.OnChange.Subscribe(func(c signal.Change[ExhibitID]) {
		if idx := exhibitIndex(c.New); idx >= 0 && n.drawerID.Get() != idx {
			n.drawerID.Set(idx)
		}
	})
	indexOpenID := n.shell.IndexOpen.OnChange.Subscribe(func(signal.Change[bool]) {
		layout.PropagateContentDirty(n, ctx.Runtime, "narrow.indexOpen", facet.DirtyLayout|facet.DirtyProjection)
	})
	inspectorOpenID := n.shell.InspectorOpen.OnChange.Subscribe(func(signal.Change[bool]) {
		layout.PropagateContentDirty(n, ctx.Runtime, "narrow.inspectorOpen", facet.DirtyLayout|facet.DirtyProjection)
	})
	n.cleanup = func() {
		n.drawer.Activated.Unsubscribe(drawerID)
		n.shell.ActiveExhibit.OnChange.Unsubscribe(activeID)
		n.shell.IndexOpen.OnChange.Unsubscribe(indexOpenID)
		n.shell.InspectorOpen.OnChange.Unsubscribe(inspectorOpenID)
	}
}

func (n *NarrowShell) OnDetach() {
	if n.cleanup != nil {
		n.cleanup()
		n.cleanup = nil
	}
}

func (n *NarrowShell) setActive(id ExhibitID) {
	if n.shell.ActiveExhibit.Get() == id {
		return
	}
	// No manual layout routing: the store write re-lays the stage (structural)
	// and the drawer/rail re-project their selection via the version-tracked
	// ActiveIndex stores (RX-1 FR-3).
	n.shell.ActiveExhibit.Set(id)
}

// Drawer returns the nav_drawer mark.
func (n *NarrowShell) Drawer() *navigation.NavDrawer { return n.drawer }

// Scrim returns the hit-blocking scrim mark.
func (n *NarrowShell) Scrim() *feedback.Scrim { return n.scrim }

// Rail returns the bottom action bar (horizontal nav_rail).
func (n *NarrowShell) Rail() *navigation.NavRail { return n.bar }

// Sheet returns the inspector bottom sheet.
func (n *NarrowShell) Sheet() *ExhibitInspector { return n.sheet }

// DrawerIndex returns the drawer/rail active-index store.
func (n *NarrowShell) DrawerIndex() *store.ValueStore[int] { return n.drawerID }

func (n *NarrowShell) Base() *facet.Facet { n.BindImpl(n); return &n.Facet }
func (n *NarrowShell) OnActivate()        {}
func (n *NarrowShell) OnDeactivate()      {}
