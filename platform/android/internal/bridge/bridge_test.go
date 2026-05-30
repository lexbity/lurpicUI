//go:build !android
// +build !android

package bridge

import (
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/platform"
)

func TestEventQueue_PushAndPoll(t *testing.T) {
	q := NewEventQueue()

	// Initially empty
	events := q.Poll()
	if len(events) != 0 {
		t.Errorf("expected empty queue, got %d events", len(events))
	}

	// Push an event
	event := Event{Type: EventTypeStart}
	q.Push(event)

	// Poll should return the event
	events = q.Poll()
	if len(events) != 1 {
		t.Errorf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != EventTypeStart {
		t.Errorf("expected EventTypeStart, got %v", events[0].Type)
	}

	// Queue should be empty again
	events = q.Poll()
	if len(events) != 0 {
		t.Errorf("expected empty queue after poll, got %d events", len(events))
	}
}

func TestEventQueue_Wait(t *testing.T) {
	q := NewEventQueue()

	// Push an event in background
	go func() {
		time.Sleep(10 * time.Millisecond)
		q.Push(Event{Type: EventTypeResume})
	}()

	// Wait should return when event is available
	events := q.Wait()
	if len(events) != 1 {
		t.Errorf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != EventTypeResume {
		t.Errorf("expected EventTypeResume, got %v", events[0].Type)
	}
}

func TestEventQueue_Close(t *testing.T) {
	q := NewEventQueue()

	if q.IsClosed() {
		t.Error("new queue should not be closed")
	}

	q.Close()

	if !q.IsClosed() {
		t.Error("queue should be closed after Close()")
	}

	// Push should be no-op on closed queue
	q.Push(Event{Type: EventTypeStart})
	events := q.Poll()
	if len(events) != 0 {
		t.Error("push on closed queue should not add events")
	}
}

func TestEventQueue_MultipleEvents(t *testing.T) {
	q := NewEventQueue()

	// Push multiple events
	q.Push(Event{Type: EventTypeStart})
	q.Push(Event{Type: EventTypeResume})
	q.Push(Event{Type: EventTypePause})

	events := q.Poll()
	if len(events) != 3 {
		t.Errorf("expected 3 events, got %d", len(events))
	}

	// Check order is preserved
	if events[0].Type != EventTypeStart {
		t.Errorf("event 0: expected EventTypeStart, got %v", events[0].Type)
	}
	if events[1].Type != EventTypeResume {
		t.Errorf("event 1: expected EventTypeResume, got %v", events[1].Type)
	}
	if events[2].Type != EventTypePause {
		t.Errorf("event 2: expected EventTypePause, got %v", events[2].Type)
	}
}

func TestEventQueue_TouchEvent(t *testing.T) {
	q := NewEventQueue()

	event := Event{
		Type:      EventTypeTouch,
		PointerID: 1,
		Phase:     TouchDown,
		X:         100.5,
		Y:         200.5,
		Pressure:  0.8,
		Major:     10.0,
		Minor:     8.0,
	}
	q.Push(event)

	events := q.Poll()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	e := events[0]
	if e.Type != EventTypeTouch {
		t.Errorf("expected EventTypeTouch, got %v", e.Type)
	}
	if e.PointerID != 1 {
		t.Errorf("expected PointerID 1, got %d", e.PointerID)
	}
	if e.Phase != TouchDown {
		t.Errorf("expected TouchDown, got %v", e.Phase)
	}
	if e.X != 100.5 || e.Y != 200.5 {
		t.Errorf("expected position (100.5, 200.5), got (%f, %f)", e.X, e.Y)
	}
}

func TestGetEventQueue_Singleton(t *testing.T) {
	// Get the global queue twice
	q1 := GetEventQueue()
	q2 := GetEventQueue()

	// Should be the same instance
	if q1 != q2 {
		t.Error("GetEventQueue should return the same instance")
	}
}

func TestInit(t *testing.T) {
	// Init should not panic
	Init()
}

func TestTouchEventExtendedFields(t *testing.T) {
	q := NewEventQueue()

	event := Event{
		Type:        EventTypeTouch,
		PointerID:   42,
		Phase:       TouchDown,
		X:           100.0,
		Y:           200.0,
		Pressure:    0.75,
		Major:       12.0,
		Minor:       10.0,
		Source:      0x1002,   // AINPUT_SOURCE_TOUCHSCREEN
		DeviceID:    1,
		ToolType:    1,        // AMOTION_EVENT_TOOL_TYPE_FINGER
		ButtonState: 0,
		EventTime:   1234567890,
	}
	q.Push(event)

	events := q.Poll()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	e := events[0]
	if e.Type != EventTypeTouch {
		t.Errorf("expected EventTypeTouch, got %v", e.Type)
	}
	if e.PointerID != 42 {
		t.Errorf("expected PointerID 42, got %d", e.PointerID)
	}
	if e.Phase != TouchDown {
		t.Errorf("expected TouchDown, got %v", e.Phase)
	}
	if e.X != 100.0 || e.Y != 200.0 {
		t.Errorf("expected (100, 200), got (%f, %f)", e.X, e.Y)
	}
	if e.Pressure != 0.75 {
		t.Errorf("expected Pressure 0.75, got %f", e.Pressure)
	}
	if e.Source != 0x1002 {
		t.Errorf("expected Source 0x1002 (touchscreen), got %d", e.Source)
	}
	if e.DeviceID != 1 {
		t.Errorf("expected DeviceID 1, got %d", e.DeviceID)
	}
	if e.ToolType != 1 {
		t.Errorf("expected ToolType 1 (finger), got %d", e.ToolType)
	}
	if e.EventTime != 1234567890 {
		t.Errorf("expected EventTime 1234567890, got %d", e.EventTime)
	}
}

func TestKeyEventFields(t *testing.T) {
	q := NewEventQueue()

	event := Event{
		Type:      EventTypeKey,
		KeyCode:   61,  // AKEYCODE_SPACE
		Action:    0,    // AKEY_EVENT_ACTION_DOWN
		MetaState: 1,    // AMETA_SHIFT_ON
		Key:       platform.KeySpace,
		Modifiers: platform.ModShift,
		Source:    0x101, // AINPUT_SOURCE_KEYBOARD
		DeviceID:  2,
		EventTime: 987654321,
	}
	q.Push(event)

	events := q.Poll()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	e := events[0]
	if e.Type != EventTypeKey {
		t.Errorf("expected EventTypeKey, got %v", e.Type)
	}
	if e.KeyCode != 61 {
		t.Errorf("expected KeyCode 61 (SPACE), got %d", e.KeyCode)
	}
	if e.Action != 0 {
		t.Errorf("expected Action 0 (DOWN), got %d", e.Action)
	}
	if e.Source != 0x101 {
		t.Errorf("expected Source 0x101 (keyboard), got %d", e.Source)
	}
	if e.DeviceID != 2 {
		t.Errorf("expected DeviceID 2, got %d", e.DeviceID)
	}
	if e.Key != platform.KeySpace {
		t.Errorf("expected KeySpace, got %v", e.Key)
	}
	if e.Modifiers != platform.ModShift {
		t.Errorf("expected ModShift, got %v", e.Modifiers)
	}
}

func TestTouchCancelPhase(t *testing.T) {
	q := NewEventQueue()

	q.Push(Event{Type: EventTypeTouch, PointerID: 1, Phase: TouchDown, X: 10, Y: 20})
	q.Push(Event{Type: EventTypeTouch, PointerID: 1, Phase: TouchCancel, X: 10, Y: 20})

	events := q.Poll()
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Phase != TouchDown {
		t.Errorf("event 0: expected TouchDown, got %v", events[0].Phase)
	}
	if events[1].Phase != TouchCancel {
		t.Errorf("event 1: expected TouchCancel, got %v", events[1].Phase)
	}
}

func TestMultiTouchPointerIDs(t *testing.T) {
	q := NewEventQueue()

	// Simulate two-finger gesture: finger 0 down, finger 1 down, both move, finger 1 up, finger 0 up
	q.Push(Event{Type: EventTypeTouch, PointerID: 0, Phase: TouchDown, X: 100, Y: 200})
	q.Push(Event{Type: EventTypeTouch, PointerID: 1, Phase: TouchDown, X: 300, Y: 400})
	q.Push(Event{Type: EventTypeTouch, PointerID: 0, Phase: TouchMove, X: 110, Y: 210})
	q.Push(Event{Type: EventTypeTouch, PointerID: 1, Phase: TouchMove, X: 290, Y: 390})
	q.Push(Event{Type: EventTypeTouch, PointerID: 1, Phase: TouchUp, X: 285, Y: 385})
	q.Push(Event{Type: EventTypeTouch, PointerID: 0, Phase: TouchUp, X: 115, Y: 215})

	events := q.Poll()
	if len(events) != 6 {
		t.Fatalf("expected 6 events, got %d", len(events))
	}

	expected := []struct {
		pointerID int32
		phase     TouchPhase
	}{
		{0, TouchDown},
		{1, TouchDown},
		{0, TouchMove},
		{1, TouchMove},
		{1, TouchUp},
		{0, TouchUp},
	}
	for i, exp := range expected {
		if events[i].PointerID != exp.pointerID {
			t.Errorf("event %d: expected PointerID %d, got %d", i, exp.pointerID, events[i].PointerID)
		}
		if events[i].Phase != exp.phase {
			t.Errorf("event %d: expected Phase %v, got %v", i, exp.phase, events[i].Phase)
		}
}
}
