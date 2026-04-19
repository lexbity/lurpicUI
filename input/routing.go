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
	Text string
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
func (ClickEvent) isDeliveredEvent()          {}
func (DragStartEvent) isDeliveredEvent()      {}
func (DragMoveEvent) isDeliveredEvent()       {}
func (DragEndEvent) isDeliveredEvent()        {}
func (FocusGainedEvent) isDeliveredEvent()    {}
func (FocusLostEvent) isDeliveredEvent()      {}

// processPointer converts a raw pointer event into routed events.
func (s *System) processPointer(e platform.EventPointer, hitMap *projection.HitMap) []RoutedEvent {
	if s == nil {
		return nil
	}
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
	hit := hitMap.HitTest(e.Position)
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
	local := s.transformToLocal(e.Position, hit.FacetID, hitMap)

	// Allow the facet's OnHitTest to refine the MarkID and filter transparent hits.
	if s.focusTree != nil {
		if refined := refineHitTest(s.focusTree, hit.FacetID, local); refined != nil {
			if !refined.Hit {
				// OnHitTest reports nothing at this position; treat as a miss.
				ptr.PressTarget = nil
				return nil
			}
			hit.MarkID = refined.MarkID
		}
	}

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

	if hitMap == nil {
		ptr.Position = e.Position
		return nil
	}
	hit := hitMap.HitTest(e.Position)
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
		local := s.transformToLocal(e.Position, hit.FacetID, hitMap)
		out = append(out, RoutedEvent{Target: hit.FacetID, Event: PointerEnterEvent{Position: local, ScreenPos: e.Position, MarkID: hit.MarkID}})
	}
	local := s.transformToLocal(e.Position, hit.FacetID, hitMap)
	out = append(out, RoutedEvent{Target: hit.FacetID, Event: PointerMoveEvent{Position: local, ScreenPos: e.Position, Modifiers: e.Modifiers, MarkID: hit.MarkID}})
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

func (s *System) processScroll(e platform.EventScroll, hitMap *projection.HitMap) []RoutedEvent {
	if s == nil {
		return nil
	}
	ptr := s.getOrCreatePointer(0)
	var targetID facet.FacetID
	var markID facet.MarkID
	if ptr != nil && ptr.PressTarget != nil && ptr.DragActive {
		targetID = ptr.PressTarget.FacetID
		markID = ptr.PressTarget.MarkID
	} else if hitMap != nil {
		if hit := hitMap.HitTest(e.Position); hit != nil {
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

func (s *System) processKey(e platform.EventKey) []RoutedEvent {
	if s == nil {
		return nil
	}
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
		case platform.EventScroll:
			routed = append(routed, s.processScroll(ev, hitMap)...)
		case platform.EventKey:
			routed = append(routed, s.processKey(ev)...)
		case platform.EventText:
			routed = append(routed, s.processText(ev)...)
		case platform.EventWindowFocus:
			if !ev.Focused {
				s.ClearPointerState()
				s.focus.Clear()
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
	base := root.Base()
	if base == nil {
		return nil
	}
	if base.ID() == target {
		return []facet.FacetImpl{root}
	}
	for _, child := range base.Children() {
		if child == nil {
			continue
		}
		if path := findFacetPath(child, target); len(path) > 0 {
			return append([]facet.FacetImpl{root}, path...)
		}
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
		return role.OnText(facet.TextEvent{Text: ev.Text})
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
	case PointerEnterEvent, PointerLeaveEvent, HoverSettledEvent, FocusGainedEvent, FocusLostEvent:
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
