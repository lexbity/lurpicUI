package input

import (
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/projection"
)

func (s *System) TickHover(now time.Time) []RoutedEvent {
	if s == nil {
		return nil
	}
	if !s.hoverEnabled {
		return nil
	}
	routed := s.hover.Tick(now, s.config)
	for i := range routed {
		routed[i].Source = s.lastPointerSource
		routed[i].Attribution = s.lastPointerAttribution
	}
	return routed
}

func (s *System) Process(events []platform.Event, hitMap *projection.HitMap, tree facet.FacetImpl) []RoutedEvent {
	if s == nil {
		return nil
	}
	s.focusTree = tree
	var routed []RoutedEvent
	for _, e := range events {
		switch e.(type) {
		case platform.EventPointer, platform.TouchEvent:
			s.lastPointerSource = platform.EventSourceOf(e)
			s.lastPointerAttribution = platform.InputAttributionOf(e)
		}
		start := len(routed)
		switch ev := e.(type) {
		case platform.EventPointer:
			routed = append(routed, s.processPointer(ev, hitMap)...)
		case platform.TouchEvent:
			routed = append(routed, s.processTouch(ev, hitMap)...)
		case platform.EventScroll:
			routed = append(routed, s.processScroll(ev, hitMap)...)
		case platform.EventKey:
			routed = append(routed, s.processKey(ev)...)
		case platform.EventText:
			routed = append(routed, s.processText(ev)...)
		case platform.EventIMECompose:
			routed = append(routed, s.processIMEText(ev.Text, true)...)
		case platform.EventIMECommit:
			routed = append(routed, s.processIMEText(ev.Text, false)...)
		case platform.EventWindowFocus:
			if !ev.Focused {
				s.ClearPointerState()
				s.focus.Clear()
				s.SetInputModality(facet.InputModalityUnknown)
			}
		}
		for i := start; i < len(routed); i++ {
			routed[i].Source = platform.EventSourceOf(e)
			routed[i].Attribution = platform.InputAttributionOf(e)
		}
	}
	return routed
}

// transformToLocal converts a screen point to local coordinates using the hit map transform.
func (s *System) transformToLocal(screenPt gfx.Point, facetID facet.FacetID, hitMap *projection.HitMap) gfx.Point {
	if hitMap == nil {
		return screenPt
	}
	t, ok := hitMap.TransformFor(facetID)
	if !ok {
		return screenPt
	}
	inv, ok := t.Inverse()
	if !ok {
		return screenPt
	}
	return inv.TransformPoint(screenPt)
}

// deltaToLocal converts a screen-space delta into local coordinates.
func (s *System) deltaToLocal(screenDelta gfx.Point, facetID facet.FacetID, hitMap *projection.HitMap) gfx.Point {
	if hitMap == nil {
		return screenDelta
	}
	t, ok := hitMap.TransformFor(facetID)
	if !ok {
		return screenDelta
	}
	inv, ok := t.Inverse()
	if !ok {
		return screenDelta
	}
	return gfx.Point{
		X: inv.A*screenDelta.X + inv.B*screenDelta.Y,
		Y: inv.C*screenDelta.X + inv.D*screenDelta.Y,
	}
}

// Deliver routes a routed event through the facet tree and bubbles upward when appropriate.
func Deliver(event RoutedEvent, tree facet.FacetImpl) bool {
	if tree == nil || event.Event == nil {
		return false
	}
	path := findFacetPath(tree, event.Target)
	if len(path) == 0 {
		return false
	}
	if !isBubbling(event.Event) {
		return deliverEventToFacet(path[len(path)-1], event.Event, event.Source, event.Attribution)
	}
	for i := len(path) - 1; i >= 0; i-- {
		if deliverEventToFacet(path[i], event.Event, event.Source, event.Attribution) {
			return true
		}
	}
	return false
}

func findFacetPath(root facet.FacetImpl, target facet.FacetID) []facet.FacetImpl {
	if root == nil {
		return nil
	}
	type pathFrame struct {
		impl facet.FacetImpl
		next int
	}
	stack := []pathFrame{{impl: root}}
	path := make([]facet.FacetImpl, 0, 16)
	for len(stack) > 0 {
		frame := &stack[len(stack)-1]
		base := frame.impl.Base()
		if base == nil {
			stack = stack[:len(stack)-1]
			if len(path) > 0 {
				path = path[:len(path)-1]
			}
			continue
		}
		if frame.next == 0 {
			path = append(path, frame.impl)
			if base.ID() == target {
				out := make([]facet.FacetImpl, len(path))
				copy(out, path)
				return out
			}
		}
		children := base.Children()
		if frame.next >= len(children) {
			stack = stack[:len(stack)-1]
			if len(path) > 0 {
				path = path[:len(path)-1]
			}
			continue
		}
		child := children[frame.next]
		frame.next++
		if child == nil {
			continue
		}
		next := facet.FacetImpl(child)
		if impl := child.Impl(); impl != nil {
			next = impl
		}
		stack = append(stack, pathFrame{impl: next})
	}
	return nil
}

func deliverEventToFacet(target facet.FacetImpl, event DeliveredEvent, source platform.EventSource, attribution platform.InputAttribution) bool {
	if target == nil || event == nil {
		return false
	}
	base := target.Base()
	if base == nil {
		return false
	}
	switch ev := event.(type) {
	case PointerPressEvent:
		return deliverPointer(base, platform.PointerPress, ev.Position, ev.ScreenPos, ev.Button, ev.Modifiers, ev.MarkID, source, attribution)
	case PointerMoveEvent:
		return deliverPointer(base, platform.PointerMove, ev.Position, ev.ScreenPos, platform.PointerNone, ev.Modifiers, ev.MarkID, source, attribution)
	case PointerReleaseEvent:
		return deliverPointer(base, platform.PointerRelease, ev.Position, ev.ScreenPos, ev.Button, ev.Modifiers, ev.MarkID, source, attribution)
	case PointerEnterEvent:
		return deliverPointer(base, platform.PointerEnter, ev.Position, ev.ScreenPos, platform.PointerNone, 0, ev.MarkID, source, attribution)
	case PointerLeaveEvent:
		return deliverPointer(base, platform.PointerLeave, gfx.Point{}, gfx.Point{}, platform.PointerNone, 0, ev.MarkID, source, attribution)
	case PointerCancelEvent:
		return deliverPointer(base, platform.PointerCancel, gfx.Point{}, gfx.Point{}, platform.PointerNone, 0, ev.MarkID, source, attribution)
	case ClickEvent:
		return deliverPointer(base, platform.PointerRelease, ev.Position, ev.ScreenPos, ev.Button, ev.Modifiers, ev.MarkID, source, attribution)
	case DragStartEvent:
		return deliverPointer(base, platform.PointerPress, ev.StartPosition, ev.ScreenStart, ev.Button, ev.Modifiers, ev.MarkID, source, attribution)
	case DragMoveEvent:
		return deliverPointer(base, platform.PointerMove, ev.Position, ev.ScreenPos, ev.Button, ev.Modifiers, ev.MarkID, source, attribution)
	case DragEndEvent:
		return deliverPointer(base, platform.PointerRelease, ev.Position, ev.ScreenPos, ev.Button, ev.Modifiers, ev.MarkID, source, attribution)
	case ScrollEvent:
		role := base.InputRole()
		if role == nil || role.OnScroll == nil {
			return false
		}
		return runRecovered("scroll", base.ID(), func() bool {
			return role.OnScroll(facet.ScrollEvent{
				Position:    ev.Position,
				DeltaX:      ev.DeltaX,
				DeltaY:      ev.DeltaY,
				Precise:     ev.Precise,
				Modifiers:   ev.Modifiers,
				Source:      source,
				Attribution: attribution,
			})
		})
	case KeyInputEvent:
		role := base.InputRole()
		if role == nil || role.OnKey == nil {
			return false
		}
		return runRecovered("key", base.ID(), func() bool {
			return role.OnKey(facet.KeyEvent{
				Kind:        ev.Kind,
				Key:         ev.Key,
				Modifiers:   ev.Modifiers,
				Source:      source,
				Attribution: attribution,
			})
		})
	case TextInputEvent:
		role := base.InputRole()
		if role == nil || role.OnText == nil {
			return false
		}
		return runRecovered("text", base.ID(), func() bool {
			return role.OnText(facet.TextEvent{Text: ev.Text, Composing: ev.Composing, Source: source, Attribution: attribution})
		})
	case DismissEvent:
		role := base.InputRole()
		if role == nil || role.OnDismiss == nil {
			return false
		}
		return runRecovered("dismiss", base.ID(), func() bool {
			return role.OnDismiss(facet.DismissEvent{
				Trigger:     ev.Trigger,
				ScreenPos:   ev.ScreenPos,
				HitFacetID:  ev.HitFacetID,
				HitMarkID:   ev.HitMarkID,
				HitLayerID:  ev.HitLayerID,
				HitOrder:    ev.HitOrder,
				Source:      source,
				Attribution: attribution,
			})
		})
	case TouchInputEvent:
		role := base.InputRole()
		if role == nil || role.OnTouch == nil {
			return false
		}
		return runRecovered("touch", base.ID(), func() bool {
			touch := ev.Event
			touch.Source = source
			touch.Attribution = attribution
			return role.OnTouch(touch)
		})
	case FocusGainedEvent:
		if role := base.FocusRole(); role != nil && role.OnFocusGained != nil {
			return runRecovered("focus_gained", base.ID(), func() bool {
				role.OnFocusGained()
				return true
			})
		}
		return false
	case FocusLostEvent:
		if role := base.FocusRole(); role != nil && role.OnFocusLost != nil {
			return runRecovered("focus_lost", base.ID(), func() bool {
				role.OnFocusLost()
				return true
			})
		}
		return false
	default:
		return false
	}
}

func deliverPointer(base *facet.Facet, kind platform.PointerEventKind, local, screen gfx.Point, button platform.PointerButton, mods platform.ModifierKeys, markID facet.MarkID, source platform.EventSource, attribution platform.InputAttribution) bool {
	role := base.InputRole()
	if role == nil || role.OnPointer == nil {
		return false
	}
	return runRecovered("pointer", base.ID(), func() bool {
		return role.OnPointer(facet.PointerEvent{
			Kind:        kind,
			Position:    local,
			ScreenPos:   screen,
			Button:      button,
			Modifiers:   mods,
			MarkID:      markID,
			Source:      source,
			Attribution: attribution,
		})
	})
}

func isBubbling(e DeliveredEvent) bool {
	switch e.(type) {
	case PointerEnterEvent, PointerLeaveEvent, PointerCancelEvent, HoverSettledEvent, FocusGainedEvent, FocusLostEvent, DismissEvent:
		return false
	default:
		return true
	}
}
