package uinav

import (
	"sync"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/annotation"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme"
	uirecipe "codeburg.org/lexbit/lurpicui/theme/recipes/uinav"
)

type MenuItem struct {
	Key      string
	Label    string
	Icon     *annotation.Icon
	Disabled bool
}

type Menu struct {
	ID       string
	Anchor   AnchorSourceRef
	Items    []MenuItem
	Open     store.Binding[bool]
	Dense    bool
	OnSelect func(string)

	base         facet.Facet
	once         sync.Once
	state        controlState
	highlight    int
	layoutRole   *facet.LayoutRole
	viewportRole *facet.ViewportRole
	projection   *facet.ProjectionRole
	hitRole      *facet.HitRole
	inputRole    *facet.InputRole
	focusRole    *facet.FocusRole
}

func init() {
	registerDescriptor(marks.Descriptor{
		Family:            marks.FamilyUINav,
		ConstructionClass: marks.ConstructionComposed,
		Type:              marks.TypeName("uinav:menu"),
		Focusable:         true,
		HitTestable:       true,
		AnchorExporting:   true,
	})
}

func (m *Menu) Base() *facet.Facet { m.ensureInit(); return &m.base }
func (m *Menu) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: marks.FamilyUINav, ConstructionClass: marks.ConstructionComposed, Type: marks.TypeName("uinav:menu"), Focusable: true, HitTestable: true, AnchorExporting: true}
}
func (m *Menu) AuthoredID() string               { return m.ID }
func (m *Menu) OnAttach(ctx facet.AttachContext) { m.syncRoles() }
func (m *Menu) OnDetach()                        {}
func (m *Menu) OnActivate()                      {}
func (m *Menu) OnDeactivate()                    {}

func (m *Menu) ensureInit() {
	m.once.Do(func() {
		ensureBase(&m.base)
		m.base.BindImpl(m)
		m.layoutRole = &facet.LayoutRole{OnMeasure: func(c facet.Constraints) gfx.Size {
			b := m.bounds()
			return gfx.Size{W: b.Width(), H: b.Height()}
		}}
		m.viewportRole = &facet.ViewportRole{Transform: gfx.Identity()}
		m.projection = &facet.ProjectionRole{OnProject: func(ctx facet.ProjectionContext) *gfx.CommandList { return m.project(ctx) }}
		m.hitRole = &facet.HitRole{OnHitTest: func(p gfx.Point) facet.HitResult {
			if m.hitBounds().Contains(p) {
				return facet.HitResult{Hit: true, Cursor: facet.CursorPointer}
			}
			return facet.HitResult{}
		}}
		m.inputRole = &facet.InputRole{
			OnPointer: func(e facet.PointerEvent) bool { return m.handlePointer(e) },
			OnKey:     func(e facet.KeyEvent) bool { return m.handleKey(e) },
		}
		m.focusRole = &facet.FocusRole{
			Focusable: func() bool { return true },
			OnFocusGained: func() {
				m.state.focused = true
				m.highlight = m.firstEnabledIndex()
			},
			OnFocusLost: func() { m.state.focused = false },
		}
		m.base.AddRole(m.layoutRole)
		m.base.AddRole(m.viewportRole)
		m.base.AddRole(m.projection)
		m.base.AddRole(m.hitRole)
		m.base.AddRole(m.inputRole)
		m.base.AddRole(m.focusRole)
		m.syncRoles()
	})
}

func (m *Menu) syncRoles() {
	syncLayout(m.layoutRole, m.bounds())
	syncViewport(m.viewportRole, gfx.Identity())
}

func (m *Menu) anchorPoint() gfx.Point {
	if pt, ok := anchorPoint(rootFacet(&m.base), m.Anchor, "bounds-center"); ok {
		return pt
	}
	return gfx.Point{}
}

func (m *Menu) bounds() gfx.Rect {
	itemH := m.itemHeight()
	return gfx.RectFromXYWH(0, 0, drawerMinWidth()-menuPadding()*2-4, float32(len(m.Items))*itemH+8)
}

func (m *Menu) hitBounds() gfx.Rect {
	if !m.Open.Get() {
		return gfx.Rect{}
	}
	origin := m.anchorPoint()
	b := m.bounds().Offset(origin.X, origin.Y+12)
	return b
}

func (m *Menu) itemHeight() float32 {
	if m.Dense {
		return menuRowHeight() - 6
	}
	return menuRowHeight()
}

func (m *Menu) firstEnabledIndex() int {
	for i, item := range m.Items {
		if !item.Disabled {
			return i
		}
	}
	return 0
}

func (m *Menu) itemBounds(i int) gfx.Rect {
	origin := m.anchorPoint()
	y := origin.Y + 12 + float32(i)*m.itemHeight()
	return gfx.RectFromXYWH(origin.X, y, 220, m.itemHeight())
}

func (m *Menu) handlePointer(e facet.PointerEvent) bool {
	if !m.Open.Get() {
		return false
	}
	if e.Kind != platform.PointerPress {
		return false
	}
	for i := range m.Items {
		if m.itemBounds(i).Contains(e.Position) {
			if !m.Items[i].Disabled {
				m.highlight = i
				m.activate(i)
			}
			return true
		}
	}
	m.Open.Set(false)
	return true
}

func (m *Menu) handleKey(e facet.KeyEvent) bool {
	if !m.Open.Get() || e.Kind != platform.KeyPress {
		return false
	}
	switch e.Key {
	case platform.KeyEscape:
		m.Open.Set(false)
		return true
	case platform.KeyDown:
		m.highlight = m.nextEnabled(1)
		return true
	case platform.KeyUp:
		m.highlight = m.nextEnabled(-1)
		return true
	case platform.KeyEnter, platform.KeySpace:
		m.activate(m.highlight)
		return true
	default:
		return false
	}
}

func (m *Menu) nextEnabled(delta int) int {
	if len(m.Items) == 0 {
		return 0
	}
	idx := m.highlight
	for i := 0; i < len(m.Items); i++ {
		idx = (idx + delta + len(m.Items)) % len(m.Items)
		if !m.Items[idx].Disabled {
			return idx
		}
	}
	return m.highlight
}

func (m *Menu) activate(i int) {
	if i < 0 || i >= len(m.Items) {
		return
	}
	if m.Items[i].Disabled {
		return
	}
	if m.OnSelect != nil {
		m.OnSelect(m.Items[i].Key)
	}
	m.Open.Set(false)
}

func (m *Menu) OnLayerSpecs() []layout.LayerSpec {
	if !m.Open.Get() {
		return nil
	}
	return []layout.LayerSpec{
		{ID: 1, Placement: layout.PlacementProjected, Measurement: layout.MeasureNonStructural, CoordSpace: layout.CoordViewport, HitPolicy: layout.HitBlockBelow, RenderOrder: 200},
	}
}

func (m *Menu) project(ctx facet.ProjectionContext) *gfx.CommandList {
	slots, _ := uirecipe.ResolveMenuRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()}, func() uirecipe.MenuVariant {
		if m.Dense {
			return uirecipe.MenuDense
		}
		return uirecipe.MenuStandard
	}())
	var list gfx.CommandList
	if !m.Open.Get() {
		return &list
	}
	origin := m.anchorPoint()
	surface := m.bounds().Offset(origin.X, origin.Y+12)
	list.Add(gfx.FillRect{Rect: surface, Brush: gfx.SolidBrush(fillColor(slots.Surface.Resolve(theme.StateDefault, theme.DefaultTokens()), gfx.Color{R: 1, G: 1, B: 1, A: 1}))})
	for i, item := range m.Items {
		rect := m.itemBounds(i)
		style := slots.Item.Resolve(theme.StateDefault, theme.DefaultTokens())
		if i == m.highlight {
			style = slots.FocusRing.Resolve(theme.StateFocused, theme.DefaultTokens())
		}
		if item.Disabled {
			style = slots.Item.Resolve(theme.StateDisabled, theme.DefaultTokens())
		}
		list.Add(gfx.FillRect{Rect: rect, Brush: gfx.SolidBrush(fillColor(style, gfx.Color{R: 1, G: 1, B: 1, A: 1}))})
	}
	return &list
}
