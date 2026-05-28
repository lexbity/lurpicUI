package store

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/internal/syncutil"
)

func TestValueStore_set_from_goroutine_panics(t *testing.T) {
	syncutil.ResetRuntimeThreadForTest()
	t.Cleanup(syncutil.ResetRuntimeThreadForTest)

	syncutil.RegisterRuntimeThread()
	s := NewValueStore(1)
	done := make(chan any, 1)
	go func() {
		defer func() { done <- recover() }()
		s.Set(2)
		done <- nil
	}()
	if recovered := <-done; recovered == nil {
		t.Fatal("expected panic from non-runtime goroutine")
	}
}
