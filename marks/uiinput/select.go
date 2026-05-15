package uiinput

import (
	"sync"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/interaction"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/text"
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
	Theme    theme.Context
	Shaper   *text.Shaper

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
func (s *Select) AuthoredID() string               { return s.ID }
func (s *Select) OnAttach(ctx facet.AttachContext) { s.syncRoles() }
func (s *Select) OnDetach()                        {}
func (s *Select) OnActivate()                      {}
func (s *Select) OnDeactivate()                    {}

// IsOpen reports whether the popup list is currently visible.
func (s *Select) IsOpen() bool {
	return s != nil && s.open
}

// TriggerBounds returns the arranged bounds for the closed select field.
func (s *Select) TriggerBounds() gfx.Rect {
	return s.triggerBounds()
}

// LayerBounds returns the full bounds that should be reserved for the control.
// When open, this includes the popup list so it can be hit-tested.
func (s *Select) LayerBounds() gfx.Rect {
	trigger := s.triggerBounds()
	if !s.open || len(s.Options) == 0 {
		return trigger
	}
	popup := s.popupBounds()
	return gfx.Rect{
		Min: gfx.Point{X: minFloat32(trigger.Min.X, popup.Min.X), Y: minFloat32(trigger.Min.Y, popup.Min.Y)},
		Max: gfx.Point{X: maxFloat32(trigger.Max.X, popup.Max.X), Y: maxFloat32(trigger.Max.Y, popup.Max.Y)},
	}
}

func (s *Select) ensureInit() {
	s.once.Do(func() {
		ensureBase(&s.base)
		s.base.BindImpl(s)
		s.layoutRole = &facet.LayoutRole{OnMeasure: func(c facet.Constraints) gfx.Size {
			bounds := s.LayerBounds()
			return gfx.Size{W: bounds.Width(), H: bounds.Height()}
		}}
		s.viewportRole = &facet.ViewportRole{Transform: gfx.Identity()}
		s.projection = &facet.ProjectionRole{OnProject: func(ctx facet.ProjectionContext) *gfx.CommandList { return s.project(ctx) }}
		s.hitRole = &facet.HitRole{OnHitTest: func(p gfx.Point) facet.HitResult {
			if s.LayerBounds().Contains(p) {
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

func (s *Select) triggerBounds() gfx.Rect {
	if s.layoutRole != nil && !s.layoutRole.ArrangedBounds.IsEmpty() {
		return s.layoutRole.ArrangedBounds
	}
	return gfx.RectFromXYWH(0, 0, selectMinWidth()+40, buttonHeight())
}

func (s *Select) bounds() gfx.Rect {
	return s.triggerBounds()
}

func (s *Select) popupBounds() gfx.Rect {
	trigger := s.triggerBounds()
	itemH := selectItemHeight()
	height := float32(len(s.Options)) * itemH
	return gfx.RectFromXYWH(trigger.Min.X, trigger.Max.Y, trigger.Width(), height)
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
	trigger := s.triggerBounds()
	popup := s.popupBounds()
	handled := false
	prevHover := s.state.hovered
	prevPressed := s.state.pressed
	prevOpen := s.open
	if interaction.HoverState(&s.state.hovered, &s.state.pressed, s.Disabled, e.Kind, true) {
		handled = true
	}
	if e.Kind == platform.PointerPress {
		if trigger.Contains(e.Position) {
			s.open = !s.open
			if s.open {
				s.highlight = s.indexOf(s.Selected.Get())
				if s.highlight < 0 {
					s.highlight = 0
				}
			}
			s.state.pressed = true
			invalidate(&s.base, facet.DirtyAll, "select-trigger-press")
			return true
		}
		if s.open && popup.Contains(e.Position) {
			idx := int((e.Position.Y - popup.Min.Y) / selectItemHeight())
			if idx >= 0 && idx < len(s.Options) {
				s.highlight = idx
			}
			s.state.pressed = true
			invalidate(&s.base, facet.DirtyAll, "select-popup-press")
			return true
		}
		s.open = false
		if prevOpen != s.open || prevHover != s.state.hovered || prevPressed != s.state.pressed {
			invalidate(&s.base, facet.DirtyAll, "select-outside-press")
		}
		return handled
	}
	if e.Kind == platform.PointerRelease && s.open && popup.Contains(e.Position) {
		idx := int((e.Position.Y - popup.Min.Y) / selectItemHeight())
		if idx >= 0 && idx < len(s.Options) {
			s.Selected.Set(s.Options[idx].Key)
			s.highlight = idx
			s.open = false
			s.state.pressed = false
			invalidate(&s.base, facet.DirtyAll, "select-popup-release")
			return true
		}
	}
	if e.Kind == platform.PointerRelease {
		s.state.pressed = false
	}
	if prevHover != s.state.hovered || prevPressed != s.state.pressed || prevOpen != s.open {
		invalidate(&s.base, facet.DirtyAll, "select-pointer-state")
	}
	return handled
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
		invalidate(&s.base, facet.DirtyAll, "select-key-escape")
		return true
	case platform.KeyEnter, platform.KeySpace:
		if !s.open {
			s.open = true
			s.highlight = s.indexOf(s.Selected.Get())
			if s.highlight < 0 {
				s.highlight = 0
			}
			invalidate(&s.base, facet.DirtyAll, "select-key-open")
			return true
		}
		s.Selected.Set(s.Options[s.highlight].Key)
		s.open = false
		invalidate(&s.base, facet.DirtyAll, "select-key-commit")
		return true
	case platform.KeyDown, platform.KeyRight:
		s.open = true
		s.highlight = (s.highlight + 1) % len(s.Options)
		invalidate(&s.base, facet.DirtyAll, "select-key-next")
		return true
	case platform.KeyUp, platform.KeyLeft:
		s.open = true
		s.highlight = (s.highlight - 1 + len(s.Options)) % len(s.Options)
		invalidate(&s.base, facet.DirtyAll, "select-key-prev")
		return true
	}
	return false
}

func (s *Select) project(ctx facet.ProjectionContext) *gfx.CommandList {
	th := s.themeContext()
	var list gfx.CommandList
	trigger := s.triggerBounds()
	fieldColor := th.Color(theme.ColorSurfaceVariant)
	if fieldColor.A == 0 {
		fieldColor = gfx.Color{R: 0.16, G: 0.17, B: 0.2, A: 1}
	}
	if s.state.hovered {
		fieldColor = fieldColor.Lerp(th.Color(theme.ColorSurface), 0.08)
	}
	if s.state.pressed || s.open {
		fieldColor = fieldColor.Lerp(th.Color(theme.ColorSurface), 0.14)
	}
	list.Add(gfx.FillRect{Rect: trigger, Brush: gfx.SolidBrush(fieldColor)})
	list.Add(gfx.StrokeRect{Rect: trigger, Brush: gfx.SolidBrush(th.Color(theme.ColorBorder))})
	if s.state.focused {
		list.Add(gfx.StrokeRect{Rect: trigger.Inset(-2, -2), Brush: gfx.SolidBrush(th.Color(theme.ColorPrimary))})
	}
	valueLabel := s.selectedLabel()
	valueColor := th.Color(theme.ColorText)
	if valueLabel == "" {
		valueLabel = "Select..."
		valueColor = th.Color(theme.ColorTextSecondary)
	}
	if s.Shaper != nil {
		drawSelectText(&list, s.Shaper, th, trigger.Min.X+12, trigger.Min.Y+10, valueLabel, valueColor)
	}
	if len(s.Options) > 0 {
		arrowColor := th.Color(theme.ColorTextSecondary)
		cx := trigger.Max.X - 16
		cy := trigger.Min.Y + trigger.Height()/2
		list.Add(gfx.DrawPoints{Points: []gfx.Point{{X: cx - 4, Y: cy - 2}, {X: cx, Y: cy + 2}, {X: cx + 4, Y: cy - 2}}, Radius: 1.25, Brush: gfx.SolidBrush(arrowColor)})
	}
	if s.open {
		popup := s.popupBounds()
		popupColor := th.Color(theme.ColorSurface)
		if popupColor.A == 0 {
			popupColor = gfx.Color{R: 0.12, G: 0.13, B: 0.16, A: 1}
		}
		list.Add(gfx.FillRect{Rect: popup, Brush: gfx.SolidBrush(popupColor)})
		list.Add(gfx.StrokeRect{Rect: popup, Brush: gfx.SolidBrush(th.Color(theme.ColorBorderStrong))})
		for i, opt := range s.Options {
			row := gfx.RectFromXYWH(popup.Min.X, popup.Min.Y+float32(i)*selectItemHeight(), popup.Width(), selectItemHeight())
			if i == s.highlight {
				list.Add(gfx.FillRect{Rect: row, Brush: gfx.SolidBrush(th.Color(theme.ColorPrimary))})
			}
			if s.Shaper != nil && opt.Label != "" {
				fg := th.Color(theme.ColorText)
				if i == s.highlight {
					fg = th.Color(theme.ColorOnPrimary)
				}
				drawSelectText(&list, s.Shaper, th, row.Min.X+12, row.Min.Y+10, opt.Label, fg)
			}
		}
	}
	return &list
}

func minFloat32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func maxFloat32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func (s *Select) selectedLabel() string {
	key := s.Selected.Get()
	if key == "" {
		return ""
	}
	for _, opt := range s.Options {
		if opt.Key == key {
			return opt.Label
		}
	}
	return key
}

func (s *Select) themeContext() theme.Context {
	if s.Theme != nil {
		return s.Theme
	}
	return theme.Default()
}

func drawSelectText(list *gfx.CommandList, shaper *text.Shaper, th theme.Context, x, y float32, label string, color gfx.Color) {
	if list == nil || shaper == nil || label == "" {
		return
	}
	layout := shaper.ShapeSimple(label, th.TextStyle(theme.TextBodyM))
	if layout == nil || len(layout.Lines) == 0 {
		return
	}
	line := layout.Lines[0]
	origin := gfx.Point{X: x, Y: y + line.Baseline}
	for _, run := range line.Runs {
		list.Add(gfx.DrawGlyphRun{Run: run, Origin: origin, Brush: gfx.SolidBrush(color)})
	}
}
