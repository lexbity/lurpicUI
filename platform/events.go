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
}
