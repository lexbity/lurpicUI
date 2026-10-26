package platform

import "time"

// EventQueue is a thread-safe FIFO queue of platform events.
//
// Push may be called from any goroutine, including native callback threads.
// Poll drains the queue without blocking.
// Wait blocks until an event is available or the timeout expires.
// A timeout of zero returns immediately; a negative timeout waits forever.
// When the queue reaches capacity, Push blocks until space becomes available.
type EventQueue interface {
	Push(Event)
	Poll() []Event
	Wait(timeout time.Duration) []Event
	// Wake returns a coalesced notification when events become available. The
	// consumer drains the queue after receiving it; the notification carries no
	// event payload and never mutates runtime state.
	Wake() <-chan struct{}
	// EnqueueDrive injects LDP-synthesized events (QP-10): the same
	// mutex-guarded FIFO, the same drain, explicit provenance. Ordering
	// relative to same-frame platform events is unspecified by design.
	EnqueueDrive(evs ...Event)
}
