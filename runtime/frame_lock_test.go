package runtime

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/render"
)

// TestRunFrame_HoldsFrameLock_ForDurationOfFrame proves the frame body holds
// the frameMu read lock for its whole duration: a write lock acquisition (the
// disposal path) blocks until the in-flight frame completes.
func TestRunFrame_HoldsFrameLock_ForDurationOfFrame(t *testing.T) {
	bf := &backendFixture{}
	rt, _ := mustLifecycleRuntime(t, bf)

	frameStarted := make(chan struct{})
	releaseFrame := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releaseFrame) }) }
	defer release()

	rt.onFrameSubmitted = func() {
		close(frameStarted)
		<-releaseFrame
	}

	frameDone := make(chan struct{})
	go func() {
		rt.RunOneFrame()
		close(frameDone)
	}()

	select {
	case <-frameStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("frame did not reach the submitted hook")
	}

	writeLockAcquired := make(chan struct{})
	go func() {
		rt.frameMu.Lock()
		defer rt.frameMu.Unlock()
		close(writeLockAcquired)
	}()

	select {
	case <-writeLockAcquired:
		t.Fatal("write lock acquired while a frame was in flight")
	case <-time.After(100 * time.Millisecond):
		// expected: the disposal path blocks until the frame finishes
	}

	release()

	select {
	case <-writeLockAcquired:
	case <-time.After(2 * time.Second):
		t.Fatal("write lock was not acquired after the frame completed")
	}
	select {
	case <-frameDone:
	case <-time.After(2 * time.Second):
		t.Fatal("RunOneFrame did not finish after the frame was released")
	}
}

// gateTickFacet blocks the runtime thread inside its tick callback, holding
// the frame read lock open so a concurrent disposal must wait.
type gateTickFacet struct {
	facet.Facet
	tick    facet.TickRole
	entered chan struct{}
	gate    chan struct{}
}

func (f *gateTickFacet) Base() *facet.Facet               { f.BindImpl(f); return &f.Facet }
func (f *gateTickFacet) OnAttach(ctx facet.AttachContext) {}
func (f *gateTickFacet) OnDetach()                        {}
func (f *gateTickFacet) OnActivate()                      {}
func (f *gateTickFacet) OnDeactivate()                    {}

func newGateTickFacet(entered, gate chan struct{}) *gateTickFacet {
	f := &gateTickFacet{Facet: facet.NewFacet(), entered: entered, gate: gate}
	f.tick.OnTick = func(dt time.Duration) {
		select {
		case f.entered <- struct{}{}:
		default:
		}
		<-f.gate
	}
	f.tick.RequestTick()
	f.AddRole(&f.tick)
	return f
}

// TestShutdown_BlocksUntilInFlightFrameCompletes drives a frame that is
// mid-tick when Shutdown is signaled and asserts Shutdown does not return —
// and disposal does not begin — until the in-flight frame completes.
func TestShutdown_BlocksUntilInFlightFrameCompletes(t *testing.T) {
	entered := make(chan struct{})
	gate := make(chan struct{})
	root := newGateTickFacet(entered, gate)
	rt := mustRuntimeTree(t, root)

	var closeGateOnce sync.Once
	closeGate := func() { closeGateOnce.Do(func() { close(gate) }) }
	defer closeGate()

	errCh := make(chan error, 1)
	go func() { errCh <- rt.Run() }()

	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("frame did not enter the tick callback")
	}

	shutdownDone := make(chan struct{})
	go func() {
		rt.Shutdown()
		close(shutdownDone)
	}()

	select {
	case <-shutdownDone:
		t.Fatal("Shutdown returned while a frame was still in flight")
	case <-time.After(100 * time.Millisecond):
		// expected: Shutdown is blocked in disposeTree waiting for the frame
	}

	closeGate()

	select {
	case <-shutdownDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Shutdown did not return after the in-flight frame completed")
	}
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after Shutdown")
	}
}

// submitSignalBackend tracks frame submissions through the atomic submit
// counter so the test can observe when the Run loop has parked in Wait.
type submitSignalBackend struct {
	backendFixture
}

func (b *submitSignalBackend) Submit(frame *render.Frame) error {
	b.backendFixture.Submit(frame)
	return b.submitErr
}

// waitForSettledSubmits polls the backend submit count until it holds steady
// for minStable, which means the Run loop has settled into frameTimer.Wait.
func waitForSettledSubmits(t *testing.T, b *submitSignalBackend, minStable time.Duration) int32 {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	var last int32 = -1
	var stableSince time.Time
	for time.Now().Before(deadline) {
		cur := b.submitCount.Load()
		if cur != last {
			last = cur
			stableSince = time.Now()
		} else if time.Since(stableSince) >= minStable {
			return last
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("submit count did not settle within 10s (last=%d)", last)
	return 0
}

// TestRun_DoesNotWaitAfterShutdownSignal asserts the Run loop drains to
// doneCh instead of running a frame after shutdown is signaled while it is
// parked in frameTimer.Wait. The loop observes the closed shutdownCh at the
// second shutdown check (or the non-blocking select on the next iteration)
// and exits without touching the tree.
func TestRun_DoesNotWaitAfterShutdownSignal(t *testing.T) {
	b := &submitSignalBackend{}
	root := facet.NewFacet()
	cfg := DefaultConfig()
	cfg.TargetFPS = 4 // short frame period; long enough to park in Wait
	cfg.LayerRegistry = testLayerRegistry(t)
	rt, err := New(cfg, nil, nil, b, &root)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}

	// A phase-1 tick hook fires on every entered frame; it must not fire
	// after shutdown is signaled.
	var frameTicks atomic.Int32
	rt.RegisterPhase1TickHook(func(time.Duration) { frameTicks.Add(1) })

	errCh := make(chan error, 1)
	go func() { errCh <- rt.Run() }()

	// The start-up invalidation schedules an immediate second frame; wait
	// until the loop is parked in frameTimer.Wait (no submits for a while)
	// before signaling shutdown.
	_ = waitForSettledSubmits(t, b, 150*time.Millisecond)
	ticksBefore := frameTicks.Load()

	rt.Shutdown()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after Shutdown")
	}

	if got := frameTicks.Load(); got != ticksBefore {
		t.Fatalf("frame tick count advanced from %d to %d after shutdown signal; Run ran a frame against the tree", ticksBefore, got)
	}
}

// TestRunFrame_AfterDisposalIsNoOp asserts the disposed-root early-out in
// runFrame. The tree is disposed without closing shutdownCh so the mid-frame
// shutdown short-circuit cannot mask the check: a post-disposal frame must be
// a no-op rather than run and render.
func TestRunFrame_AfterDisposalIsNoOp(t *testing.T) {
	bf := &backendFixture{}
	rt, _ := mustLifecycleRuntime(t, bf)

	rt.RunOneFrame()
	submits := bf.submitCount.Load()
	if submits == 0 {
		t.Fatal("expected first frame to submit")
	}

	rt.disposeTree(rt.root)
	rt.RunOneFrame()
	rt.RunOneFrame()

	if got := bf.submitCount.Load(); got != submits {
		t.Fatalf("RunOneFrame after disposal submitted %d extra frames; post-disposal frames must be no-ops", got-submits)
	}
}

// TestShutdown_ConcurrentWithRunOneFrame_NoRace stresses RunOneFrame on one
// goroutine while Shutdown fires on another. Under -race it must be clean:
// frameMu serializes the frame body against disposal, and the disposed-root
// check turns any post-disposal RunOneFrame into a no-op.
func TestShutdown_ConcurrentWithRunOneFrame_NoRace(t *testing.T) {
	bf := &backendFixture{}
	rt, _ := mustLifecycleRuntime(t, bf)

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			rt.RunOneFrame()
			time.Sleep(100 * time.Microsecond)
		}
	}()

	time.Sleep(20 * time.Millisecond)
	rt.Shutdown()
	close(stop)
	wg.Wait()
}
