//go:build !lurpic_debug

package syncutil

import "testing"

func TestOnRuntimeThread_correct_in_both_goroutines(t *testing.T) {
	resetState()
	t.Cleanup(resetState)

	if !OnRuntimeThread() {
		t.Fatal("expected runtime goroutine to report true")
	}

	done := make(chan bool, 1)
	go func() {
		done <- OnRuntimeThread()
	}()
	if got := <-done; !got {
		t.Fatal("expected spawned goroutine to report true in release build")
	}
}

func TestAssertRuntimeThread_from_other_goroutine_noop(t *testing.T) {
	resetState()
	t.Cleanup(resetState)

	RegisterRuntimeThread()
	done := make(chan struct{}, 1)
	go func() {
		AssertRuntimeThread()
		done <- struct{}{}
	}()
	<-done
}
