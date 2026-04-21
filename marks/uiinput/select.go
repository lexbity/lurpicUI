package uiinput

import (
	"sync"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/store"
	uirecipe "codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
	"codeburg.org/lexbit/lurpicui/theme"
)

// SelectOption is one option in a select control.
type SelectOption struct {
	Key   string
	Label string
}

// Select is a popup-backed single-select control.
type Select struct {
	ID       string
	Options  []SelectOption
	Selected store.Binding[string]
	Variant  SelectVariant
	Disabled bool

	base         facet.Facet
	once         sync.Once
	state        controlState
	open         bool
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
		Family:            marks.FamilyUIInput,
		ConstructionClass: marks.ConstructionComposed,
		Type:              marks.TypeName("uiinput:select"),
		Focusable:         true,
		HitTestable:       true,
	})
}

func (s *Select) Base() *facet.Facet { s.ensureInit(); return &s.base }
func (s *Select) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: marks.FamilyUIInput, ConstructionClass: marks.ConstructionComposed, Type: marks.TypeName("uiinput:select"), Focusable: true, HitTestable: true}
}
func (s *Select) AuthoredID() string { return s.ID }
func (s *Select) OnAttach(ctx facet.AttachContext) { s.syncRoles() }
func (s *Select) OnDetach() {}
func (s *Select) OnActivate() {}
func (s *Select) OnDeactivate() {}

func (s *Select) ensureInit() {
	s.once.Do(func() {
		s.base.BindImpl(s)
		s.layoutRole = &facet.LayoutRole{OnMeasure: func(c facet.Constraints) gfx.Size {
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
			Focusable: func() bool { return !s.Disabled },
			OnFocusGained: func() {
				s.state.focused = true
				s.highlight = s.indexOf(s.Selected.Get())
				if s.highlight < 0 {
					s.highlight = 0
				}
			},
			OnFocusLost: func() {
				s.state.focused = false
				s.open = false
			},
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

func (s *Select) syncRoles() {
	s.state.disabled = s.Disabled
	if idx := s.indexOf(s.Selected.Get()); idx >= 0 {
		s.highlight = idx
	}
}

func (s *Select) bounds() gfx.Rect {
	return gfx.RectFromXYWH(0, 0, 180, 36)
}

func (s *Select) popupBounds() gfx.Rect {
	height := float32(len(s.Options)) * 28
	return gfx.RectFromXYWH(0, 36, 180, height)
}

func (s *Select) indexOf(key string) int {
	for i, opt := range s.Options {
		if opt.Key == key {
			return i
		}
	}
	return -1
}

func (s *Select) handlePointer(e facet.PointerEvent) bool {
	if s.Disabled {
		return false
	}
	trigger := s.bounds()
	popup := s.popupBounds()
	if e.Kind == platform.PointerPress {
		if trigger.Contains(e.Position) {
			s.open = !s.open
			if s.open {
				s.highlight = s.indexOf(s.Selected.Get())
				if s.highlight < 0 {
					s.highlight = 0
				}
			}
			return true
		}
		if s.open && popup.Contains(e.Position) {
			idx := int((e.Position.Y - popup.Min.Y) / 28)
			if idx >= 0 && idx < len(s.Options) {
				s.highlight = idx
			}
			return true
		}
		s.open = false
		return false
	}
	if e.Kind == platform.PointerRelease && s.open && popup.Contains(e.Position) {
		idx := int((e.Position.Y - popup.Min.Y) / 28)
		if idx >= 0 && idx < len(s.Options) {
			s.Selected.Set(s.Options[idx].Key)
			s.highlight = idx
			s.open = false
			return true
		}
	}
	return false
}

func (s *Select) handleKey(e facet.KeyEvent) bool {
	if s.Disabled || !s.state.focused || e.Kind != platform.KeyPress {
		return false
	}
	if len(s.Options) == 0 {
		return false
	}
	switch e.Key {
	case platform.KeyEscape:
		s.open = false
		return true
	case platform.KeyEnter, platform.KeySpace:
		if !s.open {
			s.open = true
			s.highlight = s.indexOf(s.Selected.Get())
			if s.highlight < 0 {
				s.highlight = 0
			}
			return true
		}
		s.Selected.Set(s.Options[s.highlight].Key)
		s.open = false
		return true
	case platform.KeyDown, platform.KeyRight:
		s.open = true
		s.highlight = (s.highlight + 1) % len(s.Options)
		return true
	case platform.KeyUp, platform.KeyLeft:
		s.open = true
		s.highlight = (s.highlight - 1 + len(s.Options)) % len(s.Options)
		return true
	}
	return false
}

func (s *Select) project(ctx facet.ProjectionContext) *gfx.CommandList {
	slots, _ := uirecipe.ResolveSelectRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()}, s.Variant)
	var list gfx.CommandList
	trigger := s.bounds()
	triggerStyle := slots.Field.Resolve(s.state.interactionState(), theme.DefaultTokens())
	list.Add(gfx.FillRect{Rect: trigger, Brush: gfx.SolidBrush(fillColor(triggerStyle, gfx.Color{R: 1, G: 1, B: 1, A: 1}))})
	if s.state.focused {
		focus := slots.FocusRing.Resolve(theme.StateFocused, theme.DefaultTokens())
		if len(focus.Strokes) > 0 {
			list.Add(gfx.StrokeRect{Rect: trigger.Inset(-2, -2), Stroke: strokeStyle(focus.Strokes[0]), Brush: gfx.SolidBrush(strokeColor(focus, gfx.Color{R: 0.3, G: 0.5, B: 1, A: 1}))})
		}
	}
	if s.Selected.Get() == "" {
		list.Add(gfx.DrawPoints{Points: []gfx.Point{{X: 10, Y: 18}}, Radius: 1, Brush: gfx.SolidBrush(fillColor(slots.Value.Resolve(theme.StateDefault, theme.DefaultTokens()), gfx.Color{A: 1}))})
	}
	if s.open {
		popup := s.popupBounds()
		popupStyle := slots.Popup.Resolve(theme.StateDefault, theme.DefaultTokens())
		list.Add(gfx.FillRect{Rect: popup, Brush: gfx.SolidBrush(fillColor(popupStyle, gfx.Color{R: 0.98, G: 0.98, B: 0.99, A: 1}))})
		for i, opt := range s.Options {
			row := gfx.RectFromXYWH(0, popup.Min.Y+float32(i)*28, popup.Width(), 28)
			if i == s.highlight {
				list.Add(gfx.FillRect{Rect: row, Brush: gfx.SolidBrush(gfx.Color{R: 0.9, G: 0.94, B: 1, A: 1})})
			}
			_ = opt
		}
	}
	return &list
}
