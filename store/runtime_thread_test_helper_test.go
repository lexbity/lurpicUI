//go:build lurpic_debug && !android

package store

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/internal/syncutil"
)

func withRuntimeThread(t *testing.T, fn func()) {
	t.Helper()
	syncutil.ResetRuntimeThreadForTest()
	t.Cleanup(syncutil.ResetRuntimeThreadForTest)
	syncutil.RegisterRuntimeThread()
	fn()
}

func recoverOffRuntimeThread(t *testing.T, fn func()) (recovered any) {
	t.Helper()
	done := make(chan any, 1)
	go func() {
		defer func() {
			done <- recover()
		}()
		fn()
		done <- nil
	}()
	return <-done
}

func TestWithRuntimeThread_smoke(t *testing.T) {
	called := false
	withRuntimeThread(t, func() {
		called = true
	})
	if !called {
		t.Fatal("withRuntimeThread did not call fn")
	}
}

func TestRecoverOffRuntimeThread_smoke(t *testing.T) {
	recovered := recoverOffRuntimeThread(t, func() {
		panic("boom")
	})
	if recovered == nil {
		t.Fatal("expected recovered panic, got nil")
	}
	if recovered != "boom" {
		t.Fatalf("unexpected recovered value: %v", recovered)
	}
}

func TestRecoverOffRuntimeThread_off_thread_panics(t *testing.T) {
	// Register the runtime thread on the test's goroutine, then call Set
	// from a different goroutine via recoverOffRuntimeThread.
	withRuntimeThread(t, func() {
		s := NewValueStore(0)
		recovered := recoverOffRuntimeThread(t, func() {
			s.Set(1)
		})
		if recovered == nil {
			t.Fatal("expected panic from Set off runtime thread")
		}
	})
}
