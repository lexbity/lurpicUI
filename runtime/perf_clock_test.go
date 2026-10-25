package runtime

import (
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/gfx"
)

func TestPhaseTimingUsesWallClockInVirtual(t *testing.T) {
	engineEpoch := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	wall := engineEpoch
	clock := NewClock(func() time.Time {
		wall = wall.Add(time.Millisecond)
		return wall
	})
	if err := clock.SetVirtual(); err != nil {
		t.Fatalf("SetVirtual: %v", err)
	}
	engineBefore := clock.Now()
	rt := mustRuntimeTree(t, newLayoutCountLeaf(gfx.Size{W: 100, H: 50}))
	rt.clock = clock
	defer rt.Shutdown()
	if err := rt.start(); err != nil {
		t.Fatalf("start runtime: %v", err)
	}
	rt.runFrame(engineBefore, true, false)
	stats := rt.LastFrameStats()
	if got := clock.Now(); !got.Equal(engineBefore) {
		t.Fatalf("instrumentation advanced engine time: before=%v after=%v", engineBefore, got)
	}
	if stats.LayoutDuration <= 0 || stats.ProjectDuration <= 0 {
		t.Fatalf("virtual frame phase timings must use wall time, got layout=%s project=%s", stats.LayoutDuration, stats.ProjectDuration)
	}
	if stats.FrameNumber != 1 {
		t.Fatalf("frame number = %d, want exactly one authorized frame", stats.FrameNumber)
	}
}
