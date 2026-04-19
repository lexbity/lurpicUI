//go:build lurpic_debug

package syncutil

import (
	"strings"
	"testing"
)

func TestAssertRuntimeThread_from_registered_goroutine(t *testing.T) {
	resetState()
	t.Cleanup(resetState)

	RegisterRuntimeThread()
	AssertRuntimeThread()
}

func TestAssertRuntimeThread_from_other_goroutine(t *testing.T) {
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
