package input

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/projection"
)

func (s *System) processTouch(e platform.TouchEvent, hitMap *projection.HitMap) []RoutedEvent {
	if s == nil {
		return nil
	}
	s.SetInputModality(facet.InputModalityTouch)
	touch := s.getOrCreateTouch(e.SequenceID)
	if touch == nil {
		return nil
	}

	screenPos := gfx.Point{X: e.X, Y: e.Y}
	var targetID facet.FacetID
	var markID facet.MarkID

	switch e.Phase {
	case platform.TouchDown:
		priorActive := s.activeTouchCount()
		touch.Active = true
		touch.SequenceID = e.SequenceID
		touch.Position = screenPos
		touch.StartPosition = screenPos
		touch.ScreenStart = screenPos
		if hitMap != nil {
			if hit := s.resolveHitTarget(hitMap, screenPos); hit != nil {
				targetID = hit.FacetID
				markID = hit.MarkID
				touch.Target = hit.FacetID
				touch.MarkID = hit.MarkID
			}
		}
		touch.SyntheticPointer = false
		if touch.Target != 0 && priorActive == 0 && !touchSyntheticSuppressed(touch.Target, s.focusTree) {
			touch.SyntheticPointer = true
		}
	case platform.TouchMove, platform.TouchUp, platform.TouchCancel:
		if !touch.Active {
			return nil
		}
		touch.Position = screenPos
		targetID = touch.Target
		markID = touch.MarkID
	default:
		return nil
	}

	if targetID != 0 {
		localPos := s.transformToLocal(screenPos, targetID, hitMap)
		localStart := s.transformToLocal(touch.StartPosition, targetID, hitMap)
		routed := []RoutedEvent{{
			Target: targetID,
			Event: TouchInputEvent{Event: facet.TouchEvent{
				SequenceID:  e.SequenceID,
				Phase:       e.Phase,
				Position:    localPos,
				ScreenPos:   screenPos,
				StartPos:    localStart,
				ScreenStart: touch.ScreenStart,
				Pressure:    e.Pressure,
				MarkID:      markID,
			}},
		}}
		if touch.SyntheticPointer {
			routed = append(routed, s.syntheticPointerEvents(touch, e, hitMap)...)
		}
		if e.Phase == platform.TouchUp || e.Phase == platform.TouchCancel {
			touch.Active = false
			touch.SyntheticPointer = false
			touch.Target = 0
			touch.MarkID = 0
		}
		return routed
	}

	if touch.SyntheticPointer {
		routed := s.syntheticPointerEvents(touch, e, hitMap)
		if e.Phase == platform.TouchUp || e.Phase == platform.TouchCancel {
			touch.Active = false
			touch.SyntheticPointer = false
			touch.Target = 0
			touch.MarkID = 0
		}
		return routed
	}

	if e.Phase == platform.TouchUp || e.Phase == platform.TouchCancel {
		touch.Active = false
		touch.Target = 0
		touch.MarkID = 0
	}
	return nil
}

func (s *System) syntheticPointerEvents(touch *touchState, e platform.TouchEvent, hitMap *projection.HitMap) []RoutedEvent {
	if s == nil || touch == nil || !touch.SyntheticPointer {
		return nil
	}
	modality := s.CurrentInputModality()
	s.SetInputModality(facet.InputModalityPointer)
	ptrEvent := platform.EventPointer{Position: gfx.Point{X: e.X, Y: e.Y}}
	switch e.Phase {
	case platform.TouchDown:
		ptrEvent.Kind = platform.PointerPress
		ptrEvent.Button = platform.PointerLeft
	case platform.TouchMove:
		ptrEvent.Kind = platform.PointerMove
	case platform.TouchUp:
		ptrEvent.Kind = platform.PointerRelease
		ptrEvent.Button = platform.PointerLeft
	case platform.TouchCancel:
		s.handleCancel(s.getOrCreatePointer(0))
		touch.SyntheticPointer = false
		s.SetInputModality(modality)
		return nil
	default:
		s.SetInputModality(modality)
		return nil
	}
	routed := s.processPointer(ptrEvent, hitMap)
	s.SetInputModality(modality)
	return routed
}

func touchSyntheticSuppressed(target facet.FacetID, tree facet.FacetImpl) bool {
	if tree == nil || target == 0 {
		return false
	}
	path := findFacetPath(tree, target)
	if len(path) == 0 {
		return false
	}
	base := path[len(path)-1].Base()
	if base == nil {
		return false
	}
	role := base.InputRole()
	return role != nil && role.SuppressSyntheticPointer
}

// processPointer converts a raw pointer event into routed events.
