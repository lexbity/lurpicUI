package runtime

import (
	"errors"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/job"
)

func newTestClock() *Clock {
	epoch := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	step := 0
	return NewClock(func() time.Time {
		step++
		return epoch.Add(time.Duration(step) * time.Second)
	})
}

func TestClock_WallModeReadsWallSource(t *testing.T) {
	c := newTestClock()
	if c.Mode() != ClockWall {
		t.Fatalf("mode = %v", c.Mode())
	}
	first := c.Now()
	second := c.Now()
	if !second.After(first) {
		t.Fatalf("wall Now must advance: %v then %v", first, second)
	}
	if c.Mode().String() != "wall" {
		t.Fatalf("String() = %q", c.Mode().String())
	}
}

func TestClock_VirtualModeFreezesAndAdvances(t *testing.T) {
	c := newTestClock()
	wall := c.Now()
	if err := c.SetVirtual(); err != nil {
		t.Fatalf("SetVirtual: %v", err)
	}
	frozen := c.Now()
	if !frozen.After(wall) {
		t.Fatalf("virtual anchor %v must be at/after the last wall read %v", frozen, wall)
	}
	if c.Now() != frozen {
		t.Fatal("virtual Now must be frozen between advances")
	}
	advanced := c.Advance(1500 * time.Millisecond)
	if !advanced.Equal(frozen.Add(1500 * time.Millisecond)) {
		t.Fatalf("Advance = %v", advanced)
	}
	if !c.Now().Equal(advanced) {
		t.Fatalf("Now after advance = %v, want %v", c.Now(), advanced)
	}
	if c.Mode().String() != "virtual" {
		t.Fatalf("String() = %q", c.Mode().String())
	}
}

func TestClock_AdvanceIgnoredInWallMode(t *testing.T) {
	c := newTestClock()
	wall := c.Now()
	got := c.Advance(time.Hour)
	if got.Equal(wall) || !got.After(wall) {
		t.Fatalf("wall Advance must return the wall clock, got %v (wall %v)", got, wall)
	}
}

func TestClock_RefusesVirtualMidGesture(t *testing.T) {
	c := newTestClock()
	c.SetGestureGuard(func() bool { return true })
	if !c.GestureActive() {
		t.Fatal("gesture guard must be observed")
	}
	err := c.SetVirtual()
	if !errors.Is(err, ErrClockGestureActive) {
		t.Fatalf("SetVirtual mid-gesture = %v, want ErrClockGestureActive", err)
	}
	if c.Mode() != ClockWall {
		t.Fatal("mode must remain wall after the refusal")
	}
	// Gesture over: the switch succeeds.
	c.SetGestureGuard(func() bool { return false })
	if err := c.SetVirtual(); err != nil {
		t.Fatalf("SetVirtual after gesture: %v", err)
	}
}

func TestClock_SetWallAlwaysAllowed(t *testing.T) {
	c := newTestClock()
	if err := c.SetVirtual(); err != nil {
		t.Fatalf("SetVirtual: %v", err)
	}
	c.SetGestureGuard(func() bool { return true }) // even mid-gesture
	c.SetWall()
	if c.Mode() != ClockWall {
		t.Fatal("SetWall must always succeed")
	}
	if !c.Now().After(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("wall Now after SetWall = %v", c.Now())
	}
}

func TestClock_FrameDelta(t *testing.T) {
	c := newTestClock()
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	if got := c.FrameDelta(base); got != 0 {
		t.Fatalf("first frame delta = %v, want 0", got)
	}
	if got := c.FrameDelta(base.Add(16 * time.Millisecond)); got != 16*time.Millisecond {
		t.Fatalf("second frame delta = %v", got)
	}
	// A mode switch resets the anchor: the first frame after the switch has
	// dt = 0 instead of a wall/virtual jump.
	if err := c.SetVirtual(); err != nil {
		t.Fatalf("SetVirtual: %v", err)
	}
	t1 := c.Now().Add(time.Hour)
	if got := c.FrameDelta(t1); got != 0 {
		t.Fatalf("first virtual frame delta = %v, want 0", got)
	}
	if got := c.FrameDelta(t1.Add(33 * time.Millisecond)); got != 33*time.Millisecond {
		t.Fatalf("virtual frame delta = %v", got)
	}
}

func TestClock_InputSystemWiredToClock(t *testing.T) {
	rt := mustRuntimeTree(t, buildBenchmarkTree())
	defer rt.Shutdown()
	// The input system's time source is the runtime clock: hover settle and
	// double-click windows read engine time (Debt-4).
	rt.inputSystem.SetTimeSource(rt.clock.Now)
}

func TestClock_JobsStayCorrectUnderVirtualMode(t *testing.T) {
	// job/ scheduling keeps wall time by design (QP-5): jobs are
	// snapshot-pure and version-validated, so a wildly advancing virtual
	// clock cannot corrupt a commit. Prove it: schedule a job, advance the
	// virtual clock across its flight, drain, and require the commit to land
	// exactly once with its payload intact.
	rt := mustDriveRuntime(t)
	defer rt.Shutdown()
	if err := rt.clock.SetVirtual(); err != nil {
		t.Fatalf("SetVirtual: %v", err)
	}
	committed := make(chan string, 1)
	aj := job.BindJob(uint64(1), job.Job[int, string]{
		ID:         job.JobID(1),
		Priority:   job.PriorityInteractive,
		Snapshot:   job.NewSnapshot(0, 0),
		MemoryDecl: job.JobMemoryDecl{MaxInputBytes: 16, MaxOutputBytes: 16},
		Work: func(s job.Snapshot[int], cancel *job.CancelToken) (string, error) {
			return "row-42", nil
		},
	}, func(v string) {
		committed <- v
	})
	if err := rt.jobPool.SubmitAny(aj, func(result job.AnyResult) {}); err != nil {
		t.Fatalf("submit fixture job: %v", err)
	}
	// Advance virtual time while the job is in flight.
	rt.clock.Advance(72 * time.Hour)
	deadline := time.Now().Add(5 * time.Second)
	for {
		_ = rt.jobPool.Drain()
		select {
		case v := <-committed:
			if v != "row-42" {
				t.Fatalf("committed value = %q", v)
			}
			return
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("job never committed under virtual clock")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestClockDomainIsolation(t *testing.T) {
	wall := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := NewClock(func() time.Time { return wall })
	start := clock.Now()
	if err := clock.SetVirtual(); err != nil {
		t.Fatal(err)
	}
	virtual := clock.Now()
	clock.Advance(3 * time.Hour)
	if !clock.Now().Equal(virtual.Add(3 * time.Hour)) {
		t.Fatal("virtual advance was not exact")
	}
	clock.SetWall()
	wall = wall.Add(5 * time.Second)
	if got, want := clock.Now(), virtual.Add(3*time.Hour+5*time.Second); !got.Equal(want) {
		t.Fatalf("wall return jumped engine time: got %v want %v (initial %v)", got, want, start)
	}
	if got := clock.FrameDelta(clock.Now()); got != 0 {
		t.Fatalf("first frame after mode transition dt=%v want 0", got)
	}

	prepared, err := PrepareClock(ClockConfig{InitialMode: ClockVirtual, EpochNS: 1_800_000_000_123_456_789})
	if err != nil {
		t.Fatal(err)
	}
	root := buildBenchmarkTree()
	cfg := testConfig(t)
	cfg.LayerRegistry = testLayerRegistry(t)
	cfg.PreparedClock = prepared
	rt, err := New(cfg, nil, &testWindow{width: 800, height: 600}, &backendFixture{}, root)
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown()
	if got := prepared.View().Now(); !got.Equal(rt.Clock().Now()) {
		t.Fatalf("builder/runtime clock split: view=%v runtime=%v", got, rt.Clock().Now())
	}
	if got := rt.Clock().Now().UnixNano(); got != prepared.View().Now().UnixNano() {
		t.Fatalf("prepared epoch changed: %d", got)
	}
	rt.RunOneFrame()
	if rt.bootstrapFramePending {
		t.Fatal("bootstrap transition frame was not consumed")
	}
}
