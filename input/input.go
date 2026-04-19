package input

import (
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
)

// PointerID identifies a pointer device. Mouse is 0.
type PointerID uint32

// CaptureTarget records which facet has captured the pointer.
type CaptureTarget struct {
	FacetID facet.FacetID
	MarkID  facet.MarkID
}

// PointerState tracks the persistent state of one pointer.
type PointerState struct {
	ID            PointerID
	Position      gfx.Point
	PressedButton platform.PointerButton
	PressPosition gfx.Point
	PressTarget   *CaptureTarget
	DragActive    bool
	LastMoveTime  time.Time
	clickCount    int
}

// GestureConfig controls gesture recognition thresholds.
type GestureConfig struct {
	DragThreshold       float32
	DoubleClickInterval time.Duration
	DoubleClickRadius   float32
	HoverDelay          time.Duration
	ScrollMultiplier    float32
}

// DefaultGestureConfig returns sensible gesture defaults.
func DefaultGestureConfig() GestureConfig {
	return GestureConfig{
		DragThreshold:       4,
		DoubleClickInterval: 400 * time.Millisecond,
		DoubleClickRadius:   8,
		HoverDelay:          500 * time.Millisecond,
		ScrollMultiplier:    1,
	}
}

// DeliveredEvent is implemented by routed input events.
type DeliveredEvent interface{ isDeliveredEvent() }

// HoverSettledEvent is emitted once the pointer has stayed idle long enough.
type HoverSettledEvent struct {
	MarkID facet.MarkID
}

func (HoverSettledEvent) isDeliveredEvent() {}

// RoutedEvent couples a routed event with its target facet.
type RoutedEvent struct {
	Target facet.FacetID
	Event  DeliveredEvent
}

// HoverState tracks hover firing for the current idle period.
type HoverState struct {
	currentFacet  facet.FacetID
	currentMark   facet.MarkID
	lastMoveTime  time.Time
	firedThisIdle bool
}

// Tick returns hover events once the pointer has been idle long enough.
func (h *HoverState) Tick(now time.Time, cfg GestureConfig) []RoutedEvent {
	if h == nil || h.currentFacet == 0 || h.firedThisIdle {
		return nil
	}
	if cfg.HoverDelay > 0 && !h.lastMoveTime.IsZero() && now.Sub(h.lastMoveTime) < cfg.HoverDelay {
		return nil
	}
	h.firedThisIdle = true
	return []RoutedEvent{{
		Target: h.currentFacet,
		Event:  HoverSettledEvent{MarkID: h.currentMark},
	}}
}

// OnMove records the current hover target and resets idle firing.
func (h *HoverState) OnMove(facetID facet.FacetID, markID facet.MarkID, now time.Time) {
	if h == nil {
		return
	}
	h.currentFacet = facetID
	h.currentMark = markID
	h.lastMoveTime = now
	h.firedThisIdle = false
}

// Clear resets the hover tracker.
func (h *HoverState) Clear() {
	if h == nil {
		return
	}
	*h = HoverState{}
}

// FocusState is the input system's view of keyboard focus.
type FocusState struct {
	focused facet.FacetID
}

// SetFocused updates the cached focus target.
func (f *FocusState) SetFocused(id facet.FacetID) {
	if f == nil {
		return
	}
	f.focused = id
}

// Focused reports the cached focus target.
func (f *FocusState) Focused() facet.FacetID {
	if f == nil {
		return 0
	}
	return f.focused
}

// Clear removes the focused facet.
func (f *FocusState) Clear() {
	if f == nil {
		return
	}
	f.focused = 0
}

type clickHistory struct {
	lastPos  gfx.Point
	lastTime time.Time
	count    int
}

// System stores input gesture state without performing routing yet.
type System struct {
	config       GestureConfig
	pointers     map[PointerID]*PointerState
	focus        FocusState
	focusTree    facet.FacetImpl
	hover        HoverState
	clickHistory clickHistory
}

// NewSystem constructs an input state manager.
func NewSystem(config GestureConfig) *System {
	if config.DragThreshold == 0 {
		config.DragThreshold = DefaultGestureConfig().DragThreshold
	}
	if config.DoubleClickInterval == 0 {
		config.DoubleClickInterval = DefaultGestureConfig().DoubleClickInterval
	}
	if config.DoubleClickRadius == 0 {
		config.DoubleClickRadius = DefaultGestureConfig().DoubleClickRadius
	}
	if config.HoverDelay == 0 {
		config.HoverDelay = DefaultGestureConfig().HoverDelay
	}
	if config.ScrollMultiplier == 0 {
		config.ScrollMultiplier = DefaultGestureConfig().ScrollMultiplier
	}
	return &System{
		config:   config,
		pointers: make(map[PointerID]*PointerState),
	}
}

// ClearPointerState clears pointer capture and drag state.
func (s *System) ClearPointerState() {
	if s == nil {
		return
	}
	for _, ptr := range s.pointers {
		if ptr == nil {
			continue
		}
		ptr.PressedButton = platform.PointerNone
		ptr.PressTarget = nil
		ptr.DragActive = false
	}
	s.hover.Clear()
	s.clickHistory = clickHistory{}
}

// ClearFocus clears the cached keyboard focus.
func (s *System) ClearFocus() {
	if s == nil {
		return
	}
	s.focus.Clear()
	s.focusTree = nil
}

// getOrCreatePointer returns the state for one pointer ID.
func (s *System) getOrCreatePointer(id PointerID) *PointerState {
	if s == nil {
		return nil
	}
	if s.pointers == nil {
		s.pointers = make(map[PointerID]*PointerState)
	}
	if ptr, ok := s.pointers[id]; ok {
		return ptr
	}
	ptr := &PointerState{ID: id}
	s.pointers[id] = ptr
	return ptr
}

// resolveClickCount returns 1, 2, or 3 based on timing and distance.
func (s *System) resolveClickCount(pos gfx.Point, now time.Time) int {
	if s == nil {
		return 1
	}
	cfg := s.config
	if s.clickHistory.count == 0 {
		s.clickHistory.lastPos = pos
		s.clickHistory.lastTime = now
		s.clickHistory.count = 1
		return 1
	}
	if cfg.DoubleClickInterval > 0 && !s.clickHistory.lastTime.IsZero() && now.Sub(s.clickHistory.lastTime) > cfg.DoubleClickInterval {
		s.clickHistory.lastPos = pos
		s.clickHistory.lastTime = now
		s.clickHistory.count = 1
		return 1
	}
	if cfg.DoubleClickRadius > 0 && distanceSquared(pos, s.clickHistory.lastPos) > cfg.DoubleClickRadius*cfg.DoubleClickRadius {
		s.clickHistory.lastPos = pos
		s.clickHistory.lastTime = now
		s.clickHistory.count = 1
		return 1
	}
	if s.clickHistory.count < 3 {
		s.clickHistory.count++
	}
	s.clickHistory.lastPos = pos
	s.clickHistory.lastTime = now
	return s.clickHistory.count
}

func distanceSquared(a, b gfx.Point) float32 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	return dx*dx + dy*dy
}
