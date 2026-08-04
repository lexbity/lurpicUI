package input

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/platform"
)

func (s *System) processKey(e platform.EventKey) []RoutedEvent {
	if s == nil {
		return nil
	}
	s.SetInputModality(facet.InputModalityKeyboard)
	targetID := s.focus.Focused()
	if s.focusManager != nil && s.focusManager.Focused() != 0 {
		targetID = s.focusManager.Focused()
	}
	if targetID == 0 {
		return nil
	}
	if e.Kind == platform.KeyPress && e.Key == platform.KeyTab && s.focusManager != nil {
		oldFocus := targetID
		if e.Modifiers&platform.ModShift != 0 {
			s.focusManager.TabPrev()
		} else {
			s.focusManager.TabNext()
		}
		newFocus := s.focusManager.Focused()
		if newFocus == 0 || newFocus == oldFocus {
			return nil
		}
		s.focus.SetFocused(newFocus)
		// TabNext/TabPrev already notified OnFocusGained/OnFocusLost
		// synchronously via FocusManager.SetFocus. Emitting routed
		// FocusGainedEvent/FocusLostEvent here would fire the callbacks a
		// second time when the frame loop delivers them.
		return nil
	}
	// Without a focus manager there is no tab-traversal model; deliver Tab as
	// an ordinary key event so the focused facet may handle it itself. The
	// runtime always installs a focus manager (runtime/core.go), so the
	// focus-manager path above is the one taken in practice.
	return []RoutedEvent{{
		Target: targetID,
		Event:  KeyInputEvent{Kind: e.Kind, Key: e.Key, Modifiers: e.Modifiers},
	}}
}

func (s *System) processText(e platform.EventText) []RoutedEvent {
	if s == nil {
		return nil
	}
	s.SetInputModality(facet.InputModalityKeyboard)
	targetID := s.focus.Focused()
	if s.focusManager != nil && s.focusManager.Focused() != 0 {
		targetID = s.focusManager.Focused()
	}
	if targetID == 0 {
		return nil
	}
	return []RoutedEvent{{
		Target: targetID,
		Event:  TextInputEvent{Text: e.Text},
	}}
}

func (s *System) processIMEText(text string, composing bool) []RoutedEvent {
	if s == nil {
		return nil
	}
	s.SetInputModality(facet.InputModalityKeyboard)
	var focused facet.FacetImpl
	if s.focusManager != nil {
		focused = s.focusManager.FocusedImpl()
	}
	if focused == nil && s.focus.Focused() != 0 && s.focusTree != nil {
		if path := findFacetPath(s.focusTree, s.focus.Focused()); len(path) > 0 {
			focused = path[len(path)-1]
		}
	}
	if focused == nil || focused.Base() == nil {
		return nil
	}
	role := focused.Base().InputRole()
	if role == nil || role.OnText == nil {
		return nil
	}
	return []RoutedEvent{{
		Target: focused.Base().ID(),
		Event:  TextInputEvent{Text: text, Composing: composing},
	}}
}

// refineHitTest calls a facet's OnHitTest (if any) to resolve the per-element MarkID
// at localPos. Returns nil when the facet has no OnHitTest.
