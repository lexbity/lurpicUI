package input

import (
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/projection"
)

func (s *System) processPointer(e platform.EventPointer, hitMap *projection.HitMap) []RoutedEvent {
	if s == nil {
		return nil
	}
	s.SetInputModality(facet.InputModalityPointer)
	ptr := s.getOrCreatePointer(0)
	ptr.Position = e.Position
	ptr.LastMoveTime = time.Now()
	switch e.Kind {
	case platform.PointerPress:
		out := s.handlePress(ptr, e, hitMap)
		if len(out) > 0 && out[0].Target != 0 {
			oldFocus := s.focus.Focused()
			if focusID := s.requestFocus(out[0].Target, s.focusTree); focusID != 0 {
				if focusID != oldFocus {
					out = append(out, s.focusTransitionEvents(oldFocus, focusID)...)
				}
			}
		}
		return out
	case platform.PointerMove:
		return s.handleMove(ptr, e, hitMap)
	case platform.PointerRelease:
		return s.handleRelease(ptr, e, hitMap)
	case platform.PointerCancel:
		s.handleCancel(ptr)
		return nil
	default:
		return nil
	}
}

func (s *System) handlePress(ptr *pointerState, e platform.EventPointer, hitMap *projection.HitMap) []RoutedEvent {
	if hitMap == nil {
		ptr.PressedButton = e.Button
		ptr.PressTarget = nil
		ptr.DragActive = false
		return nil
	}
	hit := s.resolveHitTarget(hitMap, e.Position)
	if hit == nil {
		ptr.PressedButton = e.Button
		ptr.PressTarget = nil
		ptr.DragActive = false
		return nil
	}
	ptr.PressedButton = e.Button
	ptr.PressPosition = e.Position
	ptr.DragActive = false
	ptr.clickCount = s.resolveClickCount(e.Position, time.Now())
	local := hit.Local

	ptr.PressTarget = &captureTarget{FacetID: hit.FacetID, MarkID: hit.MarkID}
	return []RoutedEvent{{
		Target: hit.FacetID,
		Event: PointerPressEvent{
			Position:   local,
			ScreenPos:  e.Position,
			Button:     e.Button,
			Modifiers:  e.Modifiers,
			MarkID:     hit.MarkID,
			ClickCount: ptr.clickCount,
		},
	}}
}

func (s *System) handleMove(ptr *pointerState, e platform.EventPointer, hitMap *projection.HitMap) []RoutedEvent {
	if ptr.PressTarget != nil {
		targetID := ptr.PressTarget.FacetID
		markID := ptr.PressTarget.MarkID
		local := s.transformToLocal(e.Position, targetID, hitMap)
		screenDelta := gfx.Point{X: e.Position.X - ptr.Position.X, Y: e.Position.Y - ptr.Position.Y}
		ptr.Position = e.Position
		if !ptr.DragActive {
			dx := e.Position.X - ptr.PressPosition.X
			dy := e.Position.Y - ptr.PressPosition.Y
			if dx*dx+dy*dy >= s.config.DragThreshold*s.config.DragThreshold {
				ptr.DragActive = true
				return []RoutedEvent{
					{
						Target: targetID,
						Event: DragStartEvent{
							StartPosition: ptr.PressPosition,
							ScreenStart:   ptr.PressPosition,
							Button:        ptr.PressedButton,
							Modifiers:     e.Modifiers,
							MarkID:        markID,
						},
					},
					{
						Target: targetID,
						Event: DragMoveEvent{
							Position:    local,
							ScreenPos:   e.Position,
							Delta:       s.deltaToLocal(screenDelta, targetID, hitMap),
							ScreenDelta: screenDelta,
							Button:      ptr.PressedButton,
							Modifiers:   e.Modifiers,
							MarkID:      markID,
						},
					},
				}
			}
		}
		if ptr.DragActive {
			return []RoutedEvent{{
				Target: targetID,
				Event: DragMoveEvent{
					Position:    local,
					ScreenPos:   e.Position,
					Delta:       s.deltaToLocal(screenDelta, targetID, hitMap),
					ScreenDelta: screenDelta,
					Button:      ptr.PressedButton,
					Modifiers:   e.Modifiers,
					MarkID:      markID,
				},
			}}
		}
		return []RoutedEvent{{
			Target: targetID,
			Event: PointerMoveEvent{
				Position:  local,
				ScreenPos: e.Position,
				Modifiers: e.Modifiers,
				MarkID:    markID,
			},
		}}
	}

	if s != nil && !s.hoverEnabled {
		ptr.Position = e.Position
		return nil
	}
	if hitMap == nil {
		ptr.Position = e.Position
		return nil
	}
	hit := s.resolveHitTarget(hitMap, e.Position)
	if hit == nil {
		if s.hover.currentFacet != 0 {
			out := []RoutedEvent{{Target: s.hover.currentFacet, Event: PointerLeaveEvent{MarkID: s.hover.currentMark}}}
			s.hover.Clear()
			ptr.Position = e.Position
			return out
		}
		ptr.Position = e.Position
		return nil
	}

	out := make([]RoutedEvent, 0, 3)
	if s.hover.currentFacet != 0 && s.hover.currentFacet != hit.FacetID {
		out = append(out, RoutedEvent{Target: s.hover.currentFacet, Event: PointerLeaveEvent{MarkID: s.hover.currentMark}})
	}
	if s.hover.currentFacet != hit.FacetID {
		out = append(out, RoutedEvent{Target: hit.FacetID, Event: PointerEnterEvent{Position: hit.Local, ScreenPos: e.Position, MarkID: hit.MarkID}})
	}
	out = append(out, RoutedEvent{Target: hit.FacetID, Event: PointerMoveEvent{Position: hit.Local, ScreenPos: e.Position, Modifiers: e.Modifiers, MarkID: hit.MarkID}})
	s.hover.OnMove(hit.FacetID, hit.MarkID, time.Now())
	ptr.Position = e.Position
	return out
}

func (s *System) handleRelease(ptr *pointerState, e platform.EventPointer, hitMap *projection.HitMap) []RoutedEvent {
	if ptr.PressTarget == nil {
		ptr.Position = e.Position
		return nil
	}
	targetID := ptr.PressTarget.FacetID
	markID := ptr.PressTarget.MarkID
	local := s.transformToLocal(e.Position, targetID, hitMap)
	out := make([]RoutedEvent, 0, 1)
	if ptr.DragActive {
		out = append(out, RoutedEvent{
			Target: targetID,
			Event: DragEndEvent{
				Position:  local,
				ScreenPos: e.Position,
				Button:    ptr.PressedButton,
				Modifiers: e.Modifiers,
				MarkID:    markID,
			},
		})
	} else {
		out = append(out, RoutedEvent{
			Target: targetID,
			Event: ClickEvent{
				Position:   local,
				ScreenPos:  e.Position,
				Button:     ptr.PressedButton,
				Modifiers:  e.Modifiers,
				MarkID:     markID,
				ClickCount: ptr.clickCount,
			},
		})
	}
	ptr.PressTarget = nil
	ptr.DragActive = false
	ptr.PressedButton = platform.PointerNone
	ptr.Position = e.Position
	return out
}

func (s *System) handleCancel(ptr *pointerState) {
	if s == nil || ptr == nil {
		return
	}
	ptr.PressTarget = nil
	ptr.DragActive = false
	ptr.PressedButton = platform.PointerNone
}
