package store

import (
	"strings"
	"testing"
)

func expectProjectionPanic(t *testing.T) {
	t.Helper()
	r := recover()
	if r == nil {
		t.Fatalf("expected projection-guard panic, got none")
	}
	msg, ok := r.(string)
	if !ok || !strings.Contains(msg, "projection phase") {
		t.Fatalf("panic message %q missing \"projection phase\"", msg)
	}
}

func withActiveProjectionCheck(t *testing.T) {
	t.Helper()
	SetProjectionActiveCheck(func() bool { return true })
	t.Cleanup(func() { SetProjectionActiveCheck(nil) })
}

func TestExpectProjectionPanic_smoke(t *testing.T) {
	func() {
		defer expectProjectionPanic(t)
		panic("store: mutation during projection phase")
	}()
}

func TestWithActiveProjectionCheck_smoke(t *testing.T) {
	withActiveProjectionCheck(t)
	panicked := false
	func() {
		defer func() { panicked = recover() != nil }()
		assertNotProjecting()
	}()
	if !panicked {
		t.Fatal("expected assertNotProjecting to panic with active check")
	}
}
