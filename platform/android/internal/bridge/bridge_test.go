//go:build !android
// +build !android

package bridge

import (
	"testing"
	"time"
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

	// After Init, we should be able to get the event queue
	q := GetEventQueue()
	if q == nil {
		t.Error("GetEventQueue should not return nil after Init")
	}
}
