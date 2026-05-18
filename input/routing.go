package input

import (
	"sort"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/projection"
)

// Pointer events.
type PointerPressEvent struct {
	Position, ScreenPos gfx.Point
	Button              platform.PointerButton
	Modifiers           platform.ModifierKeys
	MarkID              facet.MarkID
	ClickCount          int
}

type PointerMoveEvent struct {
	Position, ScreenPos gfx.Point
	Modifiers           platform.ModifierKeys
	MarkID              facet.MarkID
}

type PointerReleaseEvent struct {
	Position, ScreenPos gfx.Point
	Button              platform.PointerButton
	Modifiers           platform.ModifierKeys
	MarkID              facet.MarkID
}

type PointerEnterEvent struct {
	Position, ScreenPos gfx.Point
	MarkID              facet.MarkID
}

type PointerLeaveEvent struct {
	MarkID facet.MarkID
}

// Gesture events.
type ClickEvent struct {
	Position, ScreenPos gfx.Point
	Button              platform.PointerButton
	Modifiers           platform.ModifierKeys
	MarkID              facet.MarkID
	ClickCount          int
}

type DragStartEvent struct {
	StartPosition, ScreenStart gfx.Point
	Button                     platform.PointerButton
	Modifiers                  platform.ModifierKeys
	MarkID                     facet.MarkID
}

type DragMoveEvent struct {
	Position, ScreenPos gfx.Point
	Delta, ScreenDelta  gfx.Point
	Button              platform.PointerButton
	Modifiers           platform.ModifierKeys
	MarkID              facet.MarkID
}

type DragEndEvent struct {
	Position, ScreenPos gfx.Point
	Button              platform.PointerButton
	Modifiers           platform.ModifierKeys
	MarkID              facet.MarkID
}

type ScrollEvent struct {
	Position  gfx.Point
	DeltaX    float32
	DeltaY    float32
	Precise   bool
	Modifiers platform.ModifierKeys
}

type KeyInputEvent struct {
	Kind      platform.KeyEventKind
	Key       platform.Key
	Modifiers platform.ModifierKeys
}

type TextInputEvent struct {
	Text      string
	Composing bool
}

type DismissEvent struct {
	Trigger    facet.DismissalTrigger
	ScreenPos  gfx.Point
	HitFacetID facet.FacetID
	HitMarkID  facet.MarkID
	HitLayerID facet.LayerID
	HitOrder   int
}

type resolvedHitTarget struct {
	FacetID facet.FacetID
	MarkID  facet.MarkID
	Local   gfx.Point
}

type TouchInputEvent struct {
	Event facet.TouchEvent
}

type FocusGainedEvent struct{}
type FocusLostEvent struct{}

func (PointerPressEvent) isDeliveredEvent()   {}
func (PointerMoveEvent) isDeliveredEvent()    {}
func (PointerReleaseEvent) isDeliveredEvent() {}
func (PointerEnterEvent) isDeliveredEvent()   {}
func (PointerLeaveEvent) isDeliveredEvent()   {}
func (ScrollEvent) isDeliveredEvent()         {}
func (KeyInputEvent) isDeliveredEvent()       {}
func (TextInputEvent) isDeliveredEvent()      {}
func (DismissEvent) isDeliveredEvent()        {}
func (TouchInputEvent) isDeliveredEvent()     {}
func (ClickEvent) isDeliveredEvent()          {}
func (DragStartEvent) isDeliveredEvent()      {}
func (DragMoveEvent) isDeliveredEvent()       {}
func (DragEndEvent) isDeliveredEvent()        {}
func (FocusGainedEvent) isDeliveredEvent()    {}
func (FocusLostEvent) isDeliveredEvent()      {}

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

func (s *System) syntheticPointerEvents(touch *TouchState, e platform.TouchEvent, hitMap *projection.HitMap) []RoutedEvent {
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
	default:
		return nil
	}
}

func (s *System) handlePress(ptr *PointerState, e platform.EventPointer, hitMap *projection.HitMap) []RoutedEvent {
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

	ptr.PressTarget = &CaptureTarget{FacetID: hit.FacetID, MarkID: hit.MarkID}
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

func (s *System) handleMove(ptr *PointerState, e platform.EventPointer, hitMap *projection.HitMap) []RoutedEvent {
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

func (s *System) handleRelease(ptr *PointerState, e platform.EventPointer, hitMap *projection.HitMap) []RoutedEvent {
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

func (s *System) handleCancel(ptr *PointerState) {
	if s == nil || ptr == nil {
		return
	}
	ptr.PressTarget = nil
	ptr.DragActive = false
	ptr.PressedButton = platform.PointerNone
}

func (s *System) processScroll(e platform.EventScroll, hitMap *projection.HitMap) []RoutedEvent {
	if s == nil {
		return nil
	}
	s.SetInputModality(facet.InputModalityPointer)
	ptr := s.getOrCreatePointer(0)
	var targetID facet.FacetID
	var markID facet.MarkID
	if ptr != nil && ptr.PressTarget != nil && ptr.DragActive {
		targetID = ptr.PressTarget.FacetID
		markID = ptr.PressTarget.MarkID
	} else if hitMap != nil {
		if hit := s.resolveHitTarget(hitMap, e.Position); hit != nil {
			targetID = hit.FacetID
			markID = hit.MarkID
		}
	}
	if targetID == 0 {
		return nil
	}
	local := s.transformToLocal(e.Position, targetID, hitMap)
	_ = markID
	return []RoutedEvent{{
		Target: targetID,
		Event: ScrollEvent{
			Position:  local,
			DeltaX:    e.DeltaX * s.config.ScrollMultiplier,
			DeltaY:    e.DeltaY * s.config.ScrollMultiplier,
			Precise:   e.Precise,
			Modifiers: e.Modifiers,
		},
	}}
}

func (s *System) resolveHitTarget(hitMap *projection.HitMap, screenPos gfx.Point) *resolvedHitTarget {
	if s == nil || hitMap == nil {
		return nil
	}
	entries := hitMap.Entries()
	if len(entries) == 0 {
		return nil
	}
	var passthrough *resolvedHitTarget
	for _, entry := range entries {
		if entry.HitPolicy == facet.HitDisabled {
			continue
		}
		local := screenPos
		if inv, ok := entry.Transform.Inverse(); ok {
			local = inv.TransformPoint(screenPos)
		}
		if !entry.ClipRect.IsEmpty() {
			clip := entry.ClipRect
			if inv, ok := entry.Transform.Inverse(); ok {
				clip = inv.TransformRect(clip)
			}
			if !clip.Contains(local) {
				if entry.HitPolicy == facet.HitBlockBelow {
					return passthrough
				}
				continue
			}
		}
		matched := false
		for _, region := range entry.Regions {
			if !projection.HitRegionContains(region, local) {
				continue
			}
			matched = true
			markID, accepted := s.resolveHitMark(entry.FacetID, local, region.MarkID)
			if !accepted {
				if entry.HitPolicy == facet.HitBlockBelow {
					return passthrough
				}
				continue
			}
			target := &resolvedHitTarget{
				FacetID: entry.FacetID,
				MarkID:  markID,
				Local:   local,
			}
			if entry.HitPolicy == facet.HitPassThrough || region.PassThrough {
				if passthrough == nil {
					passthrough = target
				}
				continue
			}
			return target
		}
		if entry.HitPolicy == facet.HitBlockBelow {
			return passthrough
		}
		if !matched && entry.HitPolicy == facet.HitPassThrough && passthrough == nil {
			// Continue looking below. If nothing consumes, the first pass-through hit
			// stays as the fallback target.
		}
	}
	return passthrough
}

func (s *System) resolveHitMark(target facet.FacetID, localPos gfx.Point, regionMark facet.MarkID) (facet.MarkID, bool) {
	if s == nil {
		return regionMark, true
	}
	result := refineHitTest(s.focusTree, target, localPos)
	if result == nil {
		return regionMark, true
	}
	if !result.Hit {
		return 0, false
	}
	if result.MarkID != 0 {
		return result.MarkID, true
	}
	return regionMark, true
}

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
		if path := findFacetPath(s.focusTree, newFocus); len(path) > 0 {
			return s.focusTransitionEvents(oldFocus, newFocus)
		}
		return nil
	}
	out := []RoutedEvent{{
		Target: targetID,
		Event:  KeyInputEvent{Kind: e.Kind, Key: e.Key, Modifiers: e.Modifiers},
	}}
	if e.Kind == platform.KeyPress && e.Key == platform.KeyTab && !Deliver(out[0], s.focusTree) {
		out = append(out, s.handleTabNavigation(e, s.focusTree)...)
	}
	return out
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
		s.focus.SetFocused(base.ID())
		if s.focusManager != nil {
			s.focusManager.SetFocus(path[i])
		}
		s.focusTree = tree
		return base.ID()
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

func (s *System) TickHover(now time.Time) []RoutedEvent {
	if s == nil {
		return nil
	}
	if !s.hoverEnabled {
		return nil
	}
	return s.hover.Tick(now, s.config)
}

func (s *System) Process(events []platform.Event, hitMap *projection.HitMap, tree facet.FacetImpl) []RoutedEvent {
	if s == nil {
		return nil
	}
	s.focusTree = tree
	var routed []RoutedEvent
	for _, e := range events {
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
		return deliverEventToFacet(path[len(path)-1], event.Event)
	}
	for i := len(path) - 1; i >= 0; i-- {
		if deliverEventToFacet(path[i], event.Event) {
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

func deliverEventToFacet(target facet.FacetImpl, event DeliveredEvent) bool {
	if target == nil || event == nil {
		return false
	}
	base := target.Base()
	if base == nil {
		return false
	}
	switch ev := event.(type) {
	case PointerPressEvent:
		return deliverPointer(base, platform.PointerPress, ev.Position, ev.ScreenPos, ev.Button, ev.Modifiers, ev.MarkID)
	case PointerMoveEvent:
		return deliverPointer(base, platform.PointerMove, ev.Position, ev.ScreenPos, platform.PointerNone, ev.Modifiers, ev.MarkID)
	case PointerReleaseEvent:
		return deliverPointer(base, platform.PointerRelease, ev.Position, ev.ScreenPos, ev.Button, ev.Modifiers, ev.MarkID)
	case PointerEnterEvent:
		return deliverPointer(base, platform.PointerEnter, ev.Position, ev.ScreenPos, platform.PointerNone, 0, ev.MarkID)
	case PointerLeaveEvent:
		return deliverPointer(base, platform.PointerLeave, gfx.Point{}, gfx.Point{}, platform.PointerNone, 0, ev.MarkID)
	case ClickEvent:
		return deliverPointer(base, platform.PointerRelease, ev.Position, ev.ScreenPos, ev.Button, ev.Modifiers, ev.MarkID)
	case DragStartEvent:
		return deliverPointer(base, platform.PointerPress, ev.StartPosition, ev.ScreenStart, ev.Button, ev.Modifiers, ev.MarkID)
	case DragMoveEvent:
		return deliverPointer(base, platform.PointerMove, ev.Position, ev.ScreenPos, ev.Button, ev.Modifiers, ev.MarkID)
	case DragEndEvent:
		return deliverPointer(base, platform.PointerRelease, ev.Position, ev.ScreenPos, ev.Button, ev.Modifiers, ev.MarkID)
	case ScrollEvent:
		role := base.InputRole()
		if role == nil || role.OnScroll == nil {
			return false
		}
		return role.OnScroll(facet.ScrollEvent{
			Position:  ev.Position,
			DeltaX:    ev.DeltaX,
			DeltaY:    ev.DeltaY,
			Precise:   ev.Precise,
			Modifiers: ev.Modifiers,
		})
	case KeyInputEvent:
		role := base.InputRole()
		if role == nil || role.OnKey == nil {
			return false
		}
		return role.OnKey(facet.KeyEvent{
			Kind:      ev.Kind,
			Key:       ev.Key,
			Modifiers: ev.Modifiers,
		})
	case TextInputEvent:
		role := base.InputRole()
		if role == nil || role.OnText == nil {
			return false
		}
		return role.OnText(facet.TextEvent{Text: ev.Text, Composing: ev.Composing})
	case DismissEvent:
		role := base.InputRole()
		if role == nil || role.OnDismiss == nil {
			return false
		}
		return role.OnDismiss(facet.DismissEvent{
			Trigger:    ev.Trigger,
			ScreenPos:  ev.ScreenPos,
			HitFacetID: ev.HitFacetID,
			HitMarkID:  ev.HitMarkID,
			HitLayerID: ev.HitLayerID,
			HitOrder:   ev.HitOrder,
		})
	case TouchInputEvent:
		role := base.InputRole()
		if role == nil || role.OnTouch == nil {
			return false
		}
		return role.OnTouch(ev.Event)
	case FocusGainedEvent:
		if role := base.FocusRole(); role != nil && role.OnFocusGained != nil {
			role.OnFocusGained()
			return true
		}
		return false
	case FocusLostEvent:
		if role := base.FocusRole(); role != nil && role.OnFocusLost != nil {
			role.OnFocusLost()
			return true
		}
		return false
	default:
		return false
	}
}

func deliverPointer(base *facet.Facet, kind platform.PointerEventKind, local, screen gfx.Point, button platform.PointerButton, mods platform.ModifierKeys, markID facet.MarkID) bool {
	role := base.InputRole()
	if role == nil || role.OnPointer == nil {
		return false
	}
	return role.OnPointer(facet.PointerEvent{
		Kind:      kind,
		Position:  local,
		ScreenPos: screen,
		Button:    button,
		Modifiers: mods,
		MarkID:    markID,
	})
}

func isBubbling(e DeliveredEvent) bool {
	switch e.(type) {
	case PointerEnterEvent, PointerLeaveEvent, HoverSettledEvent, FocusGainedEvent, FocusLostEvent, DismissEvent:
		return false
	default:
		return true
	}
}

func focusableFacetIDs(root facet.FacetImpl) []facet.FacetID {
	if root == nil {
		return nil
	}
	type entry struct {
		id    facet.FacetID
		index int
		order int
	}
	var entries []entry
	order := 0
	var walk func(facet.FacetImpl)
	walk = func(impl facet.FacetImpl) {
		if impl == nil {
			return
		}
		base := impl.Base()
		if base == nil {
			return
		}
		if role := base.FocusRole(); role != nil {
			focusable := true
			if role.Focusable != nil {
				focusable = role.Focusable()
			}
			if focusable && role.TabIndex >= 0 {
				entries = append(entries, entry{id: base.ID(), index: role.TabIndex, order: order})
			}
		}
		order++
		for _, child := range base.Children() {
			walk(child)
		}
	}
	walk(root)
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].index != entries[j].index {
			return entries[i].index < entries[j].index
		}
		return entries[i].order < entries[j].order
	})
	ids := make([]facet.FacetID, len(entries))
	for i, e := range entries {
		ids[i] = e.id
	}
	return ids
}
