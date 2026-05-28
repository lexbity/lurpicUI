package syncutil

import (
	"strings"
	"testing"
)

func resetState() {
	runtimeGoroutineID.Store(0)
}

func TestRegisterRuntimeThread_idempotent_panics(t *testing.T) {
	resetState()
	t.Cleanup(resetState)

	RegisterRuntimeThread()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	RegisterRuntimeThread()
}

func TestAssertRuntimeThread_before_register_is_allowed(t *testing.T) {
	resetState()
	t.Cleanup(resetState)

	AssertRuntimeThread()
}

func TestCurrentGoroutineID_stable(t *testing.T) {
	resetState()
	t.Cleanup(resetState)

	id1 := currentGoroutineID()
	id2 := currentGoroutineID()
	if id1 == 0 || id2 == 0 {
		t.Fatalf("expected non-zero goroutine ids, got %d and %d", id1, id2)
	}
	if id1 != id2 {
		t.Fatalf("expected stable goroutine id, got %d and %d", id1, id2)
	}
}

func TestAssertRuntimeThread_from_registered_goroutine(t *testing.T) {
	resetState()
	t.Cleanup(resetState)

	RegisterRuntimeThread()
	AssertRuntimeThread()
}

func TestAssertRuntimeThread_from_other_goroutine_panics(t *testing.T) {
	resetState()
	t.Cleanup(resetState)

	RegisterRuntimeThread()
	done := make(chan any, 1)
	go func() {
		defer func() { done <- recover() }()
		AssertRuntimeThread()
		done <- nil
	}()

	if recovered := <-done; recovered == nil {
		t.Fatal("expected panic from other goroutine")
	}
}

func TestOnRuntimeThread_correct_in_both_goroutines(t *testing.T) {
	resetState()
	t.Cleanup(resetState)

	RegisterRuntimeThread()
	if !OnRuntimeThread() {
		t.Fatal("expected runtime goroutine to report true")
	}

	done := make(chan bool, 1)
	go func() {
		done <- OnRuntimeThread()
	}()
	if got := <-done; got {
		t.Fatal("expected spawned goroutine to report false")
	}
}

func TestAssertRuntimeThread_panics_with_descriptive_message(t *testing.T) {
	resetState()
	t.Cleanup(resetState)

	RegisterRuntimeThread()
	done := make(chan string, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				if msg, ok := r.(string); ok {
					done <- msg
					return
				}
			}
			done <- ""
		}()
		AssertRuntimeThread()
	}()
	msg := <-done
	if msg == "" {
		t.Fatal("expected panic message")
	}
	if want := "non-runtime goroutine"; !strings.Contains(msg, want) {
		t.Fatalf("panic message %q missing %q", msg, want)
	}
}
