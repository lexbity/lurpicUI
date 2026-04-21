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

type SpeedDialAction struct {
	Key   string
	Label string
	Icon  *annotation.Icon
}

type SpeedDial struct {
	ID       string
	Open     store.Binding[bool]
	Actions  []SpeedDialAction
	Anchor   AnchorSourceRef
	OnAction func(string)

	base         facet.Facet
	once         sync.Once
	state        controlState
	layoutRole   *facet.LayoutRole
	viewportRole *facet.ViewportRole
	projection   *facet.ProjectionRole
	hitRole      *facet.HitRole
	inputRole    *facet.InputRole
	focusRole    *facet.FocusRole
	highlight    int
}

func init() {
	registerDescriptor(marks.Descriptor{
		Family:            marks.FamilyUINav,
		ConstructionClass: marks.ConstructionComposed,
		Type:              marks.TypeName("uinav:speeddial"),
		Focusable:         true,
		HitTestable:       true,
		AnchorExporting:   true,
	})
}

func (s *SpeedDial) Base() *facet.Facet { s.ensureInit(); return &s.base }
func (s *SpeedDial) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: marks.FamilyUINav, ConstructionClass: marks.ConstructionComposed, Type: marks.TypeName("uinav:speeddial"), Focusable: true, HitTestable: true, AnchorExporting: true}
}
func (s *SpeedDial) AuthoredID() string               { return s.ID }
func (s *SpeedDial) OnAttach(ctx facet.AttachContext) { s.syncRoles() }
func (s *SpeedDial) OnDetach()                        {}
func (s *SpeedDial) OnActivate()                      {}
func (s *SpeedDial) OnDeactivate()                    {}

func (s *SpeedDial) ensureInit() {
	s.once.Do(func() {
		s.base.BindImpl(s)
		s.layoutRole = &facet.LayoutRole{OnMeasure: func(c facet.Constraints) gfx.Size {
			b := s.bounds()
			return gfx.Size{W: b.Width(), H: b.Height()}
		}}
		s.viewportRole = &facet.ViewportRole{Transform: gfx.Identity()}
		s.projection = &facet.ProjectionRole{OnProject: func(ctx facet.ProjectionContext) *gfx.CommandList { return s.project(ctx) }}
		s.hitRole = &facet.HitRole{OnHitTest: func(p gfx.Point) facet.HitResult {
			if s.hitBounds().Contains(p) {
				return facet.HitResult{Hit: true, Cursor: facet.CursorPointer}
			}
			return facet.HitResult{}
		}}
		s.inputRole = &facet.InputRole{
			OnPointer: func(e facet.PointerEvent) bool { return s.handlePointer(e) },
			OnKey:     func(e facet.KeyEvent) bool { return s.handleKey(e) },
		}
		s.focusRole = &facet.FocusRole{
			Focusable: func() bool { return true },
			OnFocusGained: func() {
				s.state.focused = true
				s.highlight = 0
			},
			OnFocusLost: func() { s.state.focused = false },
		}
		s.base.AddRole(s.layoutRole)
		s.base.AddRole(s.viewportRole)
		s.base.AddRole(s.projection)
		s.base.AddRole(s.hitRole)
		s.base.AddRole(s.inputRole)
		s.base.AddRole(s.focusRole)
		s.syncRoles()
	})
}

func (s *SpeedDial) syncRoles() {
	syncLayout(s.layoutRole, s.bounds())
	syncViewport(s.viewportRole, gfx.Identity())
}

func (s *SpeedDial) anchorPoint() gfx.Point {
	if pt, ok := anchorPoint(rootFacet(&s.base), s.Anchor, "bounds-center"); ok {
		return pt
	}
	return gfx.Point{}
}

func (s *SpeedDial) bounds() gfx.Rect {
	return gfx.RectFromXYWH(0, 0, 72, 72)
}

func (s *SpeedDial) hitBounds() gfx.Rect {
	if !s.Open.Get() {
		return s.fabRect()
	}
	return s.bounds().Offset(s.anchorPoint().X, s.anchorPoint().Y)
}

func (s *SpeedDial) fabRect() gfx.Rect {
	pt := s.anchorPoint()
	return gfx.RectFromXYWH(pt.X, pt.Y, 56, 56)
}

func (s *SpeedDial) actionRect(i int) gfx.Rect {
	pt := s.anchorPoint()
	y := pt.Y - float32(i+1)*60
	return gfx.RectFromXYWH(pt.X, y, 56, 56)
}

func (s *SpeedDial) handlePointer(e facet.PointerEvent) bool {
	if e.Kind != platform.PointerPress {
		return false
	}
	if s.fabRect().Contains(e.Position) {
		s.Open.Set(!s.Open.Get())
		return true
	}
	if !s.Open.Get() {
		return false
	}
	for i := range s.Actions {
		if s.actionRect(i).Contains(e.Position) {
			s.activate(i)
			return true
		}
	}
	return false
}

func (s *SpeedDial) handleKey(e facet.KeyEvent) bool {
	if !s.Open.Get() || e.Kind != platform.KeyPress || len(s.Actions) == 0 {
		return false
	}
	switch e.Key {
	case platform.KeyUp:
		s.highlight = (s.highlight - 1 + len(s.Actions)) % len(s.Actions)
		return true
	case platform.KeyDown:
		s.highlight = (s.highlight + 1) % len(s.Actions)
		return true
	case platform.KeyEnter, platform.KeySpace:
		s.activate(s.highlight)
		return true
	case platform.KeyEscape:
		s.Open.Set(false)
		return true
	default:
		return false
	}
}

func (s *SpeedDial) activate(i int) {
	if i < 0 || i >= len(s.Actions) {
		return
	}
	if s.OnAction != nil {
		s.OnAction(s.Actions[i].Key)
	}
	s.Open.Set(false)
}

func (s *SpeedDial) OnLayerSpecs() []layout.LayerSpec {
	if !s.Open.Get() {
		return []layout.LayerSpec{{ID: 1, Placement: layout.PlacementProjected, Measurement: layout.MeasureNonStructural, CoordSpace: layout.CoordViewport, HitPolicy: layout.HitNormal, RenderOrder: 200}}
	}
	return []layout.LayerSpec{
		{ID: 1, Placement: layout.PlacementProjected, Measurement: layout.MeasureNonStructural, CoordSpace: layout.CoordViewport, HitPolicy: layout.HitBlockBelow, RenderOrder: 200},
	}
}

func (s *SpeedDial) project(ctx facet.ProjectionContext) *gfx.CommandList {
	slots, _ := uirecipe.ResolveSpeedDialRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()})
	var list gfx.CommandList
	pt := s.anchorPoint()
	fab := s.fabRect()
	list.Add(gfx.FillRect{Rect: fab, Brush: gfx.SolidBrush(fillColor(slots.Fab.Resolve(theme.StateDefault, theme.DefaultTokens()), gfx.Color{R: 0.25, G: 0.45, B: 0.95, A: 1}))})
	if !s.Open.Get() {
		return &list
	}
	list.Add(gfx.FillRect{Rect: gfx.RectFromXYWH(pt.X-24, pt.Y-24, 120, float32(len(s.Actions))*60+24), Brush: gfx.SolidBrush(fillColor(slots.Backdrop.Resolve(theme.StateDefault, theme.DefaultTokens()), gfx.Color{A: 0.2}))})
	for i := range s.Actions {
		r := s.actionRect(i)
		style := slots.Action.Resolve(theme.StateDefault, theme.DefaultTokens())
		if i == s.highlight {
			style = slots.Label.Resolve(theme.StateFocused, theme.DefaultTokens())
		}
		list.Add(gfx.FillRect{Rect: r, Brush: gfx.SolidBrush(fillColor(style, gfx.Color{R: 1, G: 1, B: 1, A: 1}))})
	}
	return &list
}
