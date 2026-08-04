package testkit

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/platform"
)

// The Drive* helpers bake in the warmup-frame contract: a fresh harness has
// FrameCount == 0 (no hit map), and a Drive* call must run a warmup frame plus
// one frame per injected event.

func TestDriveClick_RunsWarmupFrameFirst(t *testing.T) {
	h := NewHarness(t, testHarnessConfig(t), newClickCounterFacet())
	if h.FrameCount != 0 {
		t.Fatalf("fresh harness FrameCount = %d, want 0", h.FrameCount)
	}
	DriveClick(h, 5, 5)
	// warmup + press frame + release frame
	if h.FrameCount < 3 {
		t.Fatalf("DriveClick ran %d frames, want >= 3 (warmup + press + release)", h.FrameCount)
	}
}

func TestDriveClick_NoWarmupIfAlreadyRun(t *testing.T) {
	h := NewHarness(t, testHarnessConfig(t), newClickCounterFacet())
	h.RunFrame()
	before := h.FrameCount
	DriveClick(h, 5, 5)
	// No double-warmup: only the press and release frames run.
	if got := h.FrameCount - before; got != 2 {
		t.Fatalf("DriveClick after an existing frame ran %d frames, want 2", got)
	}
}

func TestWarmup_Idempotent(t *testing.T) {
	h := NewHarness(t, testHarnessConfig(t), newClickCounterFacet())
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
	h := NewHarness(t, testHarnessConfig(t), newClickCounterFacet())
	DriveKeyPress(h, platform.KeyTab, 0)
	if h.FrameCount != 2 {
		t.Fatalf("DriveKeyPress ran %d frames, want 2 (warmup + event)", h.FrameCount)
	}
}

func TestDriveKeyRelease_RunsWarmupThenEventFrame(t *testing.T) {
	h := NewHarness(t, testHarnessConfig(t), newClickCounterFacet())
	DriveKeyRelease(h, platform.KeyTab, 0)
	if h.FrameCount != 2 {
		t.Fatalf("DriveKeyRelease ran %d frames, want 2 (warmup + event)", h.FrameCount)
	}
}

func TestDriveType_RunsWarmupThenEventFrame(t *testing.T) {
	h := NewHarness(t, testHarnessConfig(t), newClickCounterFacet())
	DriveType(h, "hi")
	if h.FrameCount != 2 {
		t.Fatalf("DriveType ran %d frames, want 2 (warmup + event)", h.FrameCount)
	}
}

func TestDriveScroll_RunsWarmupThenEventFrame(t *testing.T) {
	h := NewHarness(t, testHarnessConfig(t), newClickCounterFacet())
	DriveScroll(h, 10, 10, 0, -1)
	if h.FrameCount != 2 {
		t.Fatalf("DriveScroll ran %d frames, want 2 (warmup + event)", h.FrameCount)
	}
}

func TestDriveDrag_RunsWarmupThenEventPerFrame(t *testing.T) {
	h := NewHarness(t, testHarnessConfig(t), newClickCounterFacet())
	// Drag builds press + 5 moves + release = 7 events.
	if got := len(Drag(0, 0, 10, 10)); got != 7 {
		t.Fatalf("Drag events = %d, want 7", got)
	}
	DriveDrag(h, 0, 0, 10, 10)
	if h.FrameCount != 8 {
		t.Fatalf("DriveDrag ran %d frames, want 8 (warmup + 7 events)", h.FrameCount)
	}
}

func TestDriveHelpers_NilHarness_NoPanic(t *testing.T) {
	DriveClick(nil, 1, 1)
	DriveKeyPress(nil, platform.KeyTab, 0)
	DriveKeyRelease(nil, platform.KeyTab, 0)
	DriveType(nil, "x")
	DriveDrag(nil, 0, 0, 1, 1)
	DriveScroll(nil, 1, 1, 0, 0)
}
