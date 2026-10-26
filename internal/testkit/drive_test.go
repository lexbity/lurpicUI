package testkit

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/store"
)

// The Drive* helpers bake in the warmup-frame contract: a fresh harness has
// FrameCount == 0 (no hit map), and a Drive* call must run a warmup frame plus
// one frame per injected event.

func TestDriveClick_RunsWarmupFrameFirst(t *testing.T) {
	scope := store.NewScope()
	t.Cleanup(scope.Close)
	h := NewHarness(t, scope, testHarnessConfig(t), newClickCounterFacet())
	if h.FrameCount != 0 {
		t.Fatalf("fresh harness FrameCount = %d, want 0", h.FrameCount)
	}
	DriveClickAt(h, 5, 5)
	// warmup + move + press + release frames
	if h.FrameCount < 4 {
		t.Fatalf("DriveClick ran %d frames, want >= 4 (warmup + three input atoms)", h.FrameCount)
	}
}

func TestDriveClick_NoWarmupIfAlreadyRun(t *testing.T) {
	scope := store.NewScope()
	t.Cleanup(scope.Close)
	h := NewHarness(t, scope, testHarnessConfig(t), newClickCounterFacet())
	h.RunFrame()
	before := h.FrameCount
	DriveClickAt(h, 5, 5)
	// No double-warmup: move, press, and release each get their own frame.
	if got := h.FrameCount - before; got != 3 {
		t.Fatalf("DriveClick after an existing frame ran %d frames, want 3", got)
	}
}

func TestWarmup_Idempotent(t *testing.T) {
	scope := store.NewScope()
	t.Cleanup(scope.Close)
	h := NewHarness(t, scope, testHarnessConfig(t), newClickCounterFacet())
	Warmup(h)
	Warmup(h)
	if h.FrameCount != 1 {
		t.Fatalf("Warmup after first frame advanced FrameCount to %d, want 1", h.FrameCount)
	}
}

func TestWarmup_NilHarness_NoPanic(t *testing.T) {
	Warmup(nil)
}

func TestDriveKeyPress_RunsWarmupThenEventFrame(t *testing.T) {
	scope := store.NewScope()
	t.Cleanup(scope.Close)
	h := NewHarness(t, scope, testHarnessConfig(t), newClickCounterFacet())
	DriveKeyPress(h, platform.KeyTab, 0)
	if h.FrameCount != 2 {
		t.Fatalf("DriveKeyPress ran %d frames, want 2 (warmup + event)", h.FrameCount)
	}
}

func TestDriveKeyRelease_RunsWarmupThenEventFrame(t *testing.T) {
	scope := store.NewScope()
	t.Cleanup(scope.Close)
	h := NewHarness(t, scope, testHarnessConfig(t), newClickCounterFacet())
	DriveKeyRelease(h, platform.KeyTab, 0)
	if h.FrameCount != 2 {
		t.Fatalf("DriveKeyRelease ran %d frames, want 2 (warmup + event)", h.FrameCount)
	}
}

func TestDriveType_RunsWarmupThenEventFrame(t *testing.T) {
	scope := store.NewScope()
	t.Cleanup(scope.Close)
	h := NewHarness(t, scope, testHarnessConfig(t), newClickCounterFacet())
	DriveType(h, "hi")
	if h.FrameCount != 2 {
		t.Fatalf("DriveType ran %d frames, want 2 (warmup + event)", h.FrameCount)
	}
}

func TestDriveScroll_RunsWarmupThenEventFrame(t *testing.T) {
	scope := store.NewScope()
	t.Cleanup(scope.Close)
	h := NewHarness(t, scope, testHarnessConfig(t), newClickCounterFacet())
	DriveScroll(h, 10, 10, 0, -1)
	if h.FrameCount != 2 {
		t.Fatalf("DriveScroll ran %d frames, want 2 (warmup + event)", h.FrameCount)
	}
}

func TestDriveDrag_RunsWarmupThenEventPerFrame(t *testing.T) {
	scope := store.NewScope()
	t.Cleanup(scope.Close)
	h := NewHarness(t, scope, testHarnessConfig(t), newClickCounterFacet())
	// Drag builds start move + press + 5 moves + release = 8 events.
	if got := len(Drag(0, 0, 10, 10)); got != 8 {
		t.Fatalf("Drag events = %d, want 8", got)
	}
	DriveDragFromTo(h, 0, 0, 10, 10)
	if h.FrameCount != 9 {
		t.Fatalf("DriveDrag ran %d frames, want 9 (warmup + 8 events)", h.FrameCount)
	}
}

func TestDriveHelpers_NilHarness_NoPanic(t *testing.T) {
	DriveClickAt(nil, 1, 1)
	DriveKeyPress(nil, platform.KeyTab, 0)
	DriveKeyRelease(nil, platform.KeyTab, 0)
	DriveType(nil, "x")
	DriveDragFromTo(nil, 0, 0, 1, 1)
	DriveScroll(nil, 1, 1, 0, 0)
}
