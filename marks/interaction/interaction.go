package interaction

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/theme"
)

// State captures the shared hover/press/focus state used by reusable marks.
type State struct {
	Hovered      bool
	Pressed      bool
	Focused      bool
	Disabled     bool
	HoverEnabled bool
}

// ThemeState resolves the theme interaction state for the current control state.
func (s State) ThemeState() theme.InteractionState {
	if s.Disabled {
		return theme.StateDisabled
	}
	if s.Pressed {
		return theme.StatePressed
	}
	if s.HoverEnabled && s.Hovered {
		return theme.StateHover
	}
	if s.Focused {
		return theme.StateFocused
	}
	return theme.StateDefault
}

// HoverState updates hover/press booleans for a pointer event.
func HoverState(hovered, pressed *bool, disabled bool, kind platform.PointerEventKind, hoverEnabled bool) bool {
	if disabled || !hoverEnabled || hovered == nil || pressed == nil {
		return false
	}
	switch kind {
	case platform.PointerEnter, platform.PointerMove:
		*hovered = true
		return true
	case platform.PointerPress:
		*hovered = true
		*pressed = true
		return true
	case platform.PointerLeave:
		*hovered = false
		*pressed = false
		return true
	}
	return false
}

// PressReleaseState updates press booleans for activation controls.
func PressReleaseState(pressed *bool, disabled bool, e facet.PointerEvent, activate func()) bool {
	if disabled || pressed == nil {
		return false
	}
	switch e.Kind {
	case platform.PointerPress:
		*pressed = true
		return true
	case platform.PointerRelease:
		wasPressed := *pressed
		*pressed = false
		if wasPressed {
			if activate != nil {
				activate()
			}
			return true
		}
	}
	return false
}

// TogglePressReleaseState updates press booleans for toggle controls.
func TogglePressReleaseState(pressed *bool, disabled bool, e facet.PointerEvent, toggle func()) bool {
	if disabled || pressed == nil {
		return false
	}
	switch e.Kind {
	case platform.PointerPress:
		*pressed = true
		return true
	case platform.PointerRelease:
		wasPressed := *pressed
		*pressed = false
		if wasPressed {
			if toggle != nil {
				toggle()
			}
			return true
		}
	}
	return false
}

// MinimumTouchTarget returns a touch-safe minimum target size.
func MinimumTouchTarget(base float32) float32 {
	if base < 44 {
		return 44
	}
	return base
}

// TouchTargetRect expands a rect to a minimum touch target while keeping center aligned.
func TouchTargetRect(bounds gfx.Rect, minSize float32) gfx.Rect {
	if bounds.IsEmpty() {
		return bounds
	}
	if minSize < 1 {
		minSize = 44
	}
	w := bounds.Width()
	h := bounds.Height()
	if w >= minSize && h >= minSize {
		return bounds
	}
	if w < minSize {
		pad := (minSize - w) / 2
		bounds.Min.X -= pad
		bounds.Max.X += pad
	}
	if h < minSize {
		pad := (minSize - h) / 2
		bounds.Min.Y -= pad
		bounds.Max.Y += pad
	}
	return bounds
}
