package syncutil

import "testing"

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
