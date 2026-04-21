package uiinput

import (
	"codeburg.org/lexbit/lurpicui/theme"
)

type controlState struct {
	hovered  bool
	pressed  bool
	focused  bool
	disabled bool
}

type actionBinding struct {
	OnActivate func()
}

func (s *controlState) active() bool {
	return s != nil && !s.disabled
}

func (s *controlState) interactionState() theme.InteractionState {
	if s == nil {
		return theme.StateDefault
	}
	if s.disabled {
		return theme.StateDisabled
	}
	if s.pressed {
		return theme.StatePressed
	}
	if s.hovered {
		return theme.StateHover
	}
	if s.focused {
		return theme.StateFocused
	}
	return theme.StateDefault
}

