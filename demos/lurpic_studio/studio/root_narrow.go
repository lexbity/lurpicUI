package studio

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks/action"
	"codeburg.org/lexbit/lurpicui/marks/navigation"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme"
)

// NarrowShell is the narrow-mode overlay sub-tree (FR-resp): the exhibit index
// re-hosts as a nav_drawer (left edge) plus a bottom action bar of exhibit
// icons, and the inspector re-hosts as a bottom sheet. All three bind the same
// ShellState stores as the wide index/inspector panes, so a breakpoint crossing
// preserves state (F-resp: store identity, never mark pointers).
//
// The bottom bar is a bespoke horizontal host (the nav_rail mark lays its items
// out vertically and cannot be re-hosted horizontally — F-rail-shape); it is
// the mobile bottom-action-bar pattern, using icon_button destinations bound to
// the same ActiveExhibit store. The nav_rail mark itself is demonstrated in the
// E6 Navigation playground (FR-13 removed the wide index pane's rail).
type NarrowShell struct {
	facet.Facet
	layout facet.LayoutRole

	shell    *ShellState
	scrim    *narrowScrim
	drawer   *navigation.NavDrawer
	bar      *narrowRail
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
	n.scrim = newNarrowScrim(shell)

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

	// The bottom action bar is the narrow-mode exhibit selector (the mobile
	// bottom-action-bar pattern re-hosting the exhibit destinations).
	n.bar = newNarrowRail(shell)

	// The bottom sheet re-hosts the inspector (sheet mode: drag handle +
	// Escape dismissal, FR-17c).
	n.sheet = NewSheetInspector(shell, counts)

	n.AddChild(n.scrim.Base())  //lurpiclint:ignore LL021 -- the scrim is a hit-blocking overlay child, not a layer (LL021 over-fires on overlays hosted as regular children)
	n.AddChild(n.drawer.Base()) //lurpiclint:ignore LL021 -- the narrow shell hosts navigational marks as regular children, not overlays (LL021 over-fires)
	n.AddChild(n.bar.Base())    //lurpiclint:ignore LL021 -- the narrow shell hosts the action bar as a regular child, not an overlay (LL021 over-fires)
	n.AddChild(n.sheet.Base())  //lurpiclint:ignore LL021 -- the narrow shell hosts the inspector sheet as a regular child, not an overlay (LL021 over-fires)

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

// Scrim returns the hit-blocking scrim overlay.
func (n *NarrowShell) Scrim() *narrowScrim { return n.scrim }

// Rail returns the bottom action bar host.
func (n *NarrowShell) Rail() *narrowRail { return n.bar }

// Sheet returns the inspector bottom sheet.
func (n *NarrowShell) Sheet() *ExhibitInspector { return n.sheet }

// DrawerIndex returns the drawer/rail active-index store.
func (n *NarrowShell) DrawerIndex() *store.ValueStore[int] { return n.drawerID }

func (n *NarrowShell) Base() *facet.Facet { n.BindImpl(n); return &n.Facet }
func (n *NarrowShell) OnActivate()        {}
func (n *NarrowShell) OnDeactivate()      {}

// narrowRail is the bottom action bar: a horizontal host of exhibit icon
// buttons bound to the shared ActiveExhibit store (the mobile bottom-action-bar
// pattern; the nav_rail mark itself lays out vertically, F-rail-shape, and is
// demonstrated in E6).
type narrowRail struct {
	facet.Facet
	layout facet.LayoutRole
	render facet.RenderRole

	shell *ShellState
	icons []facet.FacetImpl //lurpiclint:ignore LL012 -- the hosted icon buttons are composition structure, not domain state (F-lint-hosts)
	ids   []ExhibitID       //lurpiclint:ignore LL012 -- the exhibit destinations are composition structure, not domain state (F-lint-hosts)

	background gfx.Color
	rt         facet.RuntimeServices
	cleanup    func()
}

func newNarrowRail(shell *ShellState) *narrowRail {
	r := &narrowRail{
		shell:      shell,
		background: gfx.ColorFromRGBA8(0, 0, 0, 0),
	}
	r.Facet = facet.NewFacet()
	for _, e := range exhibitCatalog {
		btn := action.NewIconButton(primitive.IconSVG(e.icon))
		r.AddChild(btn.Base()) //lurpiclint:ignore LL021 -- the narrow rail hosts action marks as regular children, not overlays (LL021 over-fires)
		r.icons = append(r.icons, btn)
		r.ids = append(r.ids, e.id)
	}
	r.layout = facet.LayoutRole{ //lurpiclint:ignore * -- bespoke bottom action bar host (F-lint-hosts)
		OnMeasure: func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
			return r.measure(ctx, c)
		},
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			r.arrange(ctx, bounds)
		},
	}
	r.layout.Child = linearChildContract(facet.StretchPolicy{Width: facet.StretchAlways, Height: facet.StretchNever})
	r.render = facet.RenderRole{
		OnCollect: func(list *gfx.CommandList, bounds gfx.Rect) {
			if r.background.A == 0 {
				return
			}
			list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(r.background)})
		},
	}
	r.AddRole(&r.layout)
	r.AddRole(&r.render)
	return r
}

func (r *narrowRail) measure(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
	h := float32(48)
	for _, icon := range r.icons {
		if role := icon.Base().LayoutRole(); role != nil {
			role.Measure(ctx, facet.Constraints{MaxSize: c.MaxSize})
			if s := role.MeasuredSize; s.H > h {
				h = s.H
			}
		}
	}
	return facet.MeasureResult{Size: gfx.Size{W: c.MaxSize.W, H: h}}
}

func (r *narrowRail) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	// F-layout-root-fallback: the runtime can select this bar as an independent
	// layout root and re-arrange it with the full window bounds when a store
	// change marks it DirtyLayout (its own ArrangedBounds was empty at that
	// instant), bypassing Root's wide-mode empty cascade. Consult the shared
	// mode flag and stay empty in wide mode no matter the supplied bounds.
	if r.shell.Mode == LayoutWide || bounds.IsEmpty() {
		for _, icon := range r.icons {
			if role := icon.Base().LayoutRole(); role != nil {
				role.Arrange(ctx, gfx.Rect{})
			}
		}
		return
	}
	// Equal columns across the width.
	n := len(r.icons)
	if n == 0 {
		return
	}
	colW := bounds.Width() / float32(n)
	for i, icon := range r.icons {
		rect := gfx.RectFromXYWH(bounds.Min.X+colW*float32(i), bounds.Min.Y, colW, bounds.Height())
		if role := icon.Base().LayoutRole(); role != nil {
			role.Arrange(ctx, rect)
		}
	}
}

func (r *narrowRail) OnAttach(ctx facet.AttachContext) {
	r.rt = ctx.Runtime
	ids := make([]signal.SubscriptionID, 0, len(r.icons))
	for i, icon := range r.icons {
		btn := icon.(*action.IconButton)
		idx := i
		ids = append(ids, btn.Activated.Subscribe(func(signal.Unit) {
			r.shell.ActiveExhibit.Set(r.ids[idx])
		}))
	}
	activeID := r.shell.ActiveExhibit.OnChange.Subscribe(func(signal.Change[ExhibitID]) {
		layout.PropagateContentDirty(r, ctx.Runtime, "narrowRail.active", facet.DirtyLayout|facet.DirtyProjection)
	})
	r.cleanup = func() {
		for i, icon := range r.icons {
			if btn, ok := icon.(*action.IconButton); ok {
				btn.Activated.Unsubscribe(ids[i])
			}
		}
		r.shell.ActiveExhibit.OnChange.Unsubscribe(activeID)
	}
}

func (r *narrowRail) OnDetach() {
	if r.cleanup != nil {
		r.cleanup()
		r.cleanup = nil
	}
}

func (r *narrowRail) Base() *facet.Facet { r.BindImpl(r); return &r.Facet }
func (r *narrowRail) OnActivate()        {}
func (r *narrowRail) OnDeactivate()      {}

// narrowScrim is the hit-blocking overlay behind the narrow drawer and bottom
// sheet (RX-1 FR-17b): it dims the stage and blocks clicks to it, and a tap on
// the scrim (outside the open overlay) dismisses it. It is a regular child of
// the NarrowShell arranged first, so the drawer/sheet paint and hit above it;
// its hit region exists only while a sheet or drawer is open.
type narrowScrim struct {
	facet.Facet
	layout facet.LayoutRole
	render facet.RenderRole
	hit    facet.HitRole
	input  facet.InputRole

	shell *ShellState
	color gfx.Color
}

func newNarrowScrim(shell *ShellState) *narrowScrim {
	s := &narrowScrim{shell: shell}
	s.Facet = facet.NewFacet()

	s.layout = facet.LayoutRole{ //lurpiclint:ignore * -- bespoke hit-blocking scrim overlay (F-lint-hosts)
		OnMeasure: func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
			return facet.MeasureResult{Size: c.Constrain(c.MaxSize)}
		},
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			if resolved, ok := ctx.Theme.(theme.ResolvedContext); ok {
				s.color = resolved.Color(theme.ColorText)
				s.color.A = 0.4
			}
		},
	}
	s.render = facet.RenderRole{
		OnCollect: func(list *gfx.CommandList, bounds gfx.Rect) {
			if bounds.IsEmpty() || !s.visible() || s.color.A == 0 {
				return
			}
			list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(s.color)})
		},
	}
	s.hit = facet.HitRole{
		OnHitTest: func(p gfx.Point) facet.HitResult {
			if !s.visible() {
				return facet.HitResult{}
			}
			b := s.layout.ArrangedBounds
			if b.IsEmpty() || !b.Contains(p) {
				return facet.HitResult{}
			}
			return facet.HitResult{Hit: true}
		},
	}
	s.input = facet.InputRole{
		OnPointer: func(e facet.PointerEvent) bool {
			if !s.visible() {
				return false
			}
			if e.Kind == platform.PointerPress && e.Button == platform.PointerLeft {
				// Tap outside the drawer/sheet: dismiss both narrow overlays.
				s.shell.IndexOpen.Set(false)
				s.shell.InspectorOpen.Set(false)
			}
			return true
		},
	}
	s.AddRole(&s.layout)
	s.AddRole(&s.render)
	s.AddRole(&s.hit)
	s.AddRole(&s.input)
	return s
}

// visible reports whether any narrow overlay is open (the scrim's render and
// hit region are gated by it).
func (s *narrowScrim) visible() bool {
	if s == nil || s.shell == nil {
		return false
	}
	return s.shell.IndexOpen.Get() || s.shell.InspectorOpen.Get()
}

func (s *narrowScrim) Base() *facet.Facet             { s.BindImpl(s); return &s.Facet }
func (s *narrowScrim) OnAttach(_ facet.AttachContext) {}
func (s *narrowScrim) OnDetach()                      {}
func (s *narrowScrim) OnActivate()                    {}
func (s *narrowScrim) OnDeactivate()                  {}
