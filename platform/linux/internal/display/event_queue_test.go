//go:build linux && cgo

package display

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/platform"
)

func newWakeTestQueue() *eventQueue {
	return &eventQueue{
		wake: make(chan struct{}, 1),
	}
}

func TestEventQueueWakeTracksQueuedEvents(t *testing.T) {
	q := newWakeTestQueue()
	q.Push(platform.EventWindowClose{})
	select {
	case <-q.Wake():
	default:
		t.Fatal("queue push did not signal readiness")
	}
	got := q.Poll()
	if len(got) != 1 {
		t.Fatalf("poll events = %d, want 1", len(got))
	}
	if _, ok := got[0].(platform.EventWindowClose); !ok {
		t.Fatalf("poll event = %T, want window close", got[0])
	}
	select {
	case <-q.Wake():
		t.Fatal("poll left a stale readiness notification")
	default:
	}
}

func TestEventQueueNativeWakeCoalescesUntilPoll(t *testing.T) {
	q := newWakeTestQueue()
	q.notifyNativeReady()
	q.notifyNativeReady()
	select {
	case <-q.Wake():
	default:
		t.Fatal("native readiness did not signal queue wake")
	}
	q.Poll()
	select {
	case <-q.Wake():
		t.Fatal("native wake was not coalesced/drained by poll")
	default:
	}
}
