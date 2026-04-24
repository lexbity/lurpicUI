package scenes

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/basic"
	"codeburg.org/lexbit/lurpicui/marks/uinav"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/ui_diagnostic_scene/scene"
)

type UINavScene struct {
	BaseScene
	th         theme.Context
	selected   store.Binding[string]
	menuOpen   store.Binding[bool]
	drawerOpen store.Binding[bool]
	page       store.Binding[int]
	offset     store.Binding[float64]
	scrollSize store.Binding[float64]
	anchor     *basic.Rect
	tabs       *uinav.Tabs
	drawer     *uinav.Drawer
	menu       *uinav.Menu
	scrollbar  *uinav.Scrollbar
	pagination *uinav.Pagination
	speedDial  *uinav.SpeedDial
}

func NewUINavScene() *UINavScene {
	s := &UINavScene{
		BaseScene: NewBaseScene(
			"uinav",
			"UI Navigation",
			"Validates tabs, drawer, menus, pagination, scrollbars, and speed-dial marks",
			[]string{"uinav"},
		),
		th:         theme.Default(),
		selected:   store.NewBinding("overview"),
		menuOpen:   store.NewBinding(true),
		drawerOpen: store.NewBinding(true),
		page:       store.NewBinding(2),
		offset:     store.NewBinding(40.0),
		scrollSize: store.NewBinding(320.0),
	}
	return s
}

func (s *UINavScene) BuildRoot() facet.FacetImpl {
	if s.root != nil {
		return s.root
	}
	col := layout.NewColumnLayout()
	col.Gap = 12
	s.root = col

	header := newTextMark("uinav-title", "Navigation primitives", 18)
	col.AddChild(header.Base())

	stack := layout.NewStackLayout(layout.AlignStart)
	col.AddChild(stack.Base())

	s.anchor = &basic.Rect{
		ID:     "uinav-anchor",
		Bounds: basic.BoundsProps{X: 120, Y: 110, W: 280, H: 180},
		Radius: 12,
		Style: basic.PrimitiveStyleProps{
			Fill:    solidFill(s.th.Color(theme.ColorSurface)),
			Stroke:  solidStroke(s.th.Color(theme.ColorPrimary), 2),
			Visible: true,
			Opacity: 1,
		},
	}
	stack.AddChild(s.anchor.Base())

	s.tabs = &uinav.Tabs{
		ID:       "uinav-tabs",
		Items:    []uinav.TabItem{{Key: "overview", Label: "Overview"}, {Key: "routes", Label: "Routes"}, {Key: "details", Label: "Details"}},
		Selected: s.selected,
		Variant:  uinav.TabsStandard,
		Theme:    s.th,
	}
	stack.AddChild(s.tabs.Base())

	s.menu = &uinav.Menu{
		ID:     "uinav-menu",
		Anchor: uinav.AnchorSourceRef{MarkID: s.anchor.ID, Anchor: "top-right"},
		Items: []uinav.MenuItem{
			{Key: "new", Label: "New"},
			{Key: "open", Label: "Open"},
			{Key: "save", Label: "Save", Disabled: true},
		},
		Open:  s.menuOpen,
		Dense: false,
	}

	s.drawer = &uinav.Drawer{
		ID:      "uinav-drawer",
		Mode:    uinav.DrawerDismissible,
		Edge:    uinav.DrawerLeft,
		Open:    s.drawerOpen,
		Content: []marks.Mark{s.menu},
	}
	stack.AddChild(s.drawer.Base())

	s.scrollbar = &uinav.Scrollbar{
		ID:          "uinav-scrollbar",
		Orientation: uinav.ScrollbarVertical,
		Viewport: uinav.ViewportBinding{
			Offset:      s.offset,
			Extent:      store.NewBinding(120.0),
			ContentSize: s.scrollSize,
		},
	}
	stack.AddChild(s.scrollbar.Base())

	s.pagination = &uinav.Pagination{
		ID:         "uinav-pagination",
		Page:       s.page,
		TotalPages: 9,
		WindowSize: 5,
	}
	stack.AddChild(s.pagination.Base())

	s.speedDial = &uinav.SpeedDial{
		ID:     "uinav-speed-dial",
		Anchor: uinav.AnchorSourceRef{MarkID: s.anchor.ID, Anchor: "bottom-right"},
		Open:   store.NewBinding(false),
		Actions: []uinav.SpeedDialAction{
			{Key: "compose", Label: "Compose"},
			{Key: "reply", Label: "Reply"},
			{Key: "share", Label: "Share"},
		},
	}
	stack.AddChild(s.speedDial.Base())

	return col
}

func (s *UINavScene) ApplyTheme(th theme.Context) {
	s.th = th
	if s.anchor != nil {
		tintRectStyle(s.anchor, th.Color(theme.ColorSurface))
	}
	if s.tabs != nil {
		s.tabs.Theme = th
	}
	if s.root != nil && s.root.Base() != nil {
		s.root.Base().Invalidate(facet.DirtyProjection)
	}
}

func (s *UINavScene) ApplyDensity(scale float32) {
	if scale <= 0 {
		scale = 1
	}
	if s.tabs != nil {
		if scale < 1 {
			s.tabs.Variant = uinav.TabsCompact
		} else {
			s.tabs.Variant = uinav.TabsStandard
		}
	}
	if s.menu != nil {
		s.menu.Dense = scale < 1
	}
	if s.root != nil && s.root.Base() != nil {
		s.root.Base().Invalidate(facet.DirtyLayout | facet.DirtyProjection)
	}
}

func (s *UINavScene) Reset() {
	s.selected.Set("overview")
	s.menuOpen.Set(true)
	s.drawerOpen.Set(true)
	s.page.Set(2)
	s.BaseScene.Reset()
}

func (s *UINavScene) ExportState() map[string]any {
	return map[string]any{
		"scene_id": s.id,
		"selected": s.selected.Get(),
		"page":     s.page.Get(),
	}
}

func (s *UINavScene) ImportState(state map[string]any) {
	if v, ok := state["selected"].(string); ok {
		s.selected.Set(v)
	}
	if v, ok := state["page"].(float64); ok {
		s.page.Set(int(v))
	}
}

var _ scene.Scene = (*UINavScene)(nil)
