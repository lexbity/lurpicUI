package input

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
)

func refineHitTest(tree facet.FacetImpl, id facet.FacetID, localPos gfx.Point) *facet.HitResult {
	path := findFacetPath(tree, id)
	if len(path) == 0 {
		return nil
	}
	hr := path[len(path)-1].Base().HitRole()
	if hr == nil || hr.OnHitTest == nil {
		return nil
	}
	result := hr.HitTest(localPos)
	return &result
}

func (s *System) requestFocus(targetID facet.FacetID, tree facet.FacetImpl) facet.FacetID {
	if s == nil || targetID == 0 || tree == nil {
		return 0
	}
	path := findFacetPath(tree, targetID)
	if len(path) == 0 {
		return 0
	}
	for i := len(path) - 1; i >= 0; i-- {
		base := path[i].Base()
		if base == nil {
			continue
		}
		role := base.FocusRole()
		if role == nil {
			continue
		}
		focusable := true
		if role.Focusable != nil {
			focusable = role.Focusable()
		}
		if !focusable {
			continue
		}
		if s.focusManager != nil {
			if !s.focusManager.SetFocus(path[i]) {
				continue
			}
			s.focus.SetFocused(s.focusManager.Focused())
		} else {
			s.focus.SetFocused(base.ID())
		}
		s.focusTree = tree
		return s.focus.Focused()
	}
	return 0
}

func (s *System) focusTransitionEvents(oldID, newID facet.FacetID) []RoutedEvent {
	if s == nil || newID == 0 {
		return nil
	}
	out := make([]RoutedEvent, 0, 2)
	if oldID != 0 && oldID != newID {
		out = append(out, RoutedEvent{Target: oldID, Event: FocusLostEvent{}})
	}
	out = append(out, RoutedEvent{Target: newID, Event: FocusGainedEvent{}})
	return out
}

func (s *System) handleTabNavigation(e platform.EventKey, tree facet.FacetImpl) []RoutedEvent {
	if s == nil || tree == nil {
		return nil
	}
	ordered := focusableFacetIDs(tree)
	if len(ordered) == 0 {
		return nil
	}
	current := s.focus.Focused()
	idx := -1
	for i, id := range ordered {
		if id == current {
			idx = i
			break
		}
	}
	if e.Modifiers&platform.ModShift != 0 {
		if idx < 0 {
			idx = len(ordered) - 1
		} else {
			idx = (idx - 1 + len(ordered)) % len(ordered)
		}
	} else {
		if idx < 0 {
			idx = 0
		} else {
			idx = (idx + 1) % len(ordered)
		}
	}
	newID := ordered[idx]
	if newID == current {
		return nil
	}
	s.focus.SetFocused(newID)
	if s.focusManager != nil {
		if path := findFacetPath(tree, newID); len(path) > 0 {
			s.focusManager.SetFocus(path[len(path)-1])
		}
	}
	s.focusTree = tree
	out := make([]RoutedEvent, 0, 2)
	if current != 0 {
		out = append(out, RoutedEvent{Target: current, Event: FocusLostEvent{}})
	}
	out = append(out, RoutedEvent{Target: newID, Event: FocusGainedEvent{}})
	return out
}
