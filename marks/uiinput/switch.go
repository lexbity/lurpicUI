package uiinput

import (
	"sync"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme"
	uirecipe "codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
)

// Switch is a toggle control with track/thumb visuals.
type Switch struct {
	ID       string
	On       store.Binding[bool]
	Label    string
	Disabled bool

	base         facet.Facet
	once         sync.Once
	state        controlState
	layoutRole   *facet.LayoutRole
	viewportRole *facet.ViewportRole
	projection   *facet.ProjectionRole
	hitRole      *facet.HitRole
	inputRole    *facet.InputRole
	focusRole    *facet.FocusRole
}

func init() {
	registerDescriptor(marks.Descriptor{
		Family:            marks.FamilyUIInput,
		ConstructionClass: marks.ConstructionComposed,
		Type:              marks.TypeName("uiinput:switch"),
		Focusable:         true,
		HitTestable:       true,
	})
}

func (s *Switch) Base() *facet.Facet { s.ensureInit(); return &s.base }
func (s *Switch) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: marks.FamilyUIInput, ConstructionClass: marks.ConstructionComposed, Type: marks.TypeName("uiinput:switch"), Focusable: true, HitTestable: true}
}
func (s *Switch) AuthoredID() string               { return s.ID }
func (s *Switch) OnAttach(ctx facet.AttachContext) { s.syncRoles() }
func (s *Switch) OnDetach()                        {}
func (s *Switch) OnActivate()                      {}
func (s *Switch) OnDeactivate()                    {}

func (s *Switch) ensureInit() {
	s.once.Do(func() {
		ensureBase(&s.base)
		s.base.BindImpl(s)
		s.layoutRole = &facet.LayoutRole{OnMeasure: func(cn facet.Constraints) gfx.Size {
			bounds := s.bounds()
			return gfx.Size{W: bounds.Width(), H: bounds.Height()}
		}}
		s.viewportRole = &facet.ViewportRole{Transform: gfx.Identity()}
		s.projection = &facet.ProjectionRole{OnProject: func(ctx facet.ProjectionContext) *gfx.CommandList { return s.project(ctx) }}
		s.hitRole = &facet.HitRole{OnHitTest: func(p gfx.Point) facet.HitResult {
			if s.bounds().Contains(p) {
				return facet.HitResult{Hit: true, Cursor: facet.CursorPointer}
			}
			return facet.HitResult{}
		}}
		s.inputRole = &facet.InputRole{
			OnPointer: func(e facet.PointerEvent) bool { return s.handlePointer(e) },
			OnKey:     func(e facet.KeyEvent) bool { return s.handleKey(e) },
		}
		s.focusRole = &facet.FocusRole{
			Focusable:     func() bool { return !s.Disabled },
			OnFocusGained: func() { s.state.focused = true },
			OnFocusLost:   func() { s.state.focused = false },
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

func (s *Switch) syncRoles() {
	s.state.disabled = s.Disabled
}

func (s *Switch) bounds() gfx.Rect {
	return gfx.RectFromXYWH(0, 0, switchTrackWidth(), switchTrackHeight())
}

func (s *Switch) handlePointer(e facet.PointerEvent) bool {
	if s.Disabled {
		return false
	}
	switch e.Kind {
	case platform.PointerPress:
		s.state.pressed = true
		return true
	case platform.PointerRelease:
		wasPressed := s.state.pressed
		s.state.pressed = false
		if wasPressed {
			s.On.Set(!s.On.Get())
			return true
		}
		return false
	default:
		return false
	}
}

func (s *Switch) handleKey(e facet.KeyEvent) bool {
	if s.Disabled || !s.state.focused || e.Kind != platform.KeyPress {
		return false
	}
	if e.Key == platform.KeyEnter || e.Key == platform.KeySpace {
		s.On.Set(!s.On.Get())
		return true
	}
	return false
}

func (s *Switch) project(ctx facet.ProjectionContext) *gfx.CommandList {
	slots, _ := uirecipe.ResolveSwitchRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()}, uirecipe.SwitchStandard)
	var list gfx.CommandList
	bounds := s.bounds()
	state := s.state.interactionState()
	track := slots.Track.Resolve(state, theme.DefaultTokens())
	thumbStyle := slots.Thumb.Resolve(state, theme.DefaultTokens())
	list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(fillColor(track, gfx.Color{R: 0.8, G: 0.8, B: 0.82, A: 1}))})
	thumb := gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, 14, 14)
	if s.On.Get() {
		thumb.Min.X = bounds.Max.X - 14
		thumb.Max.X = bounds.Max.X
	}
	list.Add(gfx.FillRect{Rect: thumb, Brush: gfx.SolidBrush(fillColor(thumbStyle, gfx.Color{R: 0.25, G: 0.45, B: 0.95, A: 1}))})
	if s.state.focused {
		focus := slots.FocusRing.Resolve(theme.StateFocused, theme.DefaultTokens())
		if len(focus.Strokes) > 0 {
			list.Add(gfx.StrokeRect{Rect: bounds.Inset(-2, -2), Stroke: strokeStyle(focus.Strokes[0]), Brush: gfx.SolidBrush(strokeColor(focus, gfx.Color{R: 0.3, G: 0.5, B: 1, A: 1}))})
		}
	}
	return &list
}
