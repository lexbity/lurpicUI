package runtime

import (
	"testing"
)

func TestOnFrameSubmitted_hook_fires_on_run_frame(t *testing.T) {
	bf := &backendFixture{}
	rt, _ := mustLifecycleRuntime(t, bf)

	fired := make(chan struct{}, 1)
	rt.onFrameSubmitted = func() {
		fired <- struct{}{}
	}

	rt.RunOneFrame()

	select {
	case <-fired:
	default:
		t.Fatal("onFrameSubmitted hook did not fire after RunOneFrame")
	}
}

func TestOnFrameSubmitted_hook_fires_multiple_times(t *testing.T) {
	bf := &backendFixture{}
	rt, _ := mustLifecycleRuntime(t, bf)

	var count int
	rt.onFrameSubmitted = func() { count++ }

	rt.RunOneFrame()
	rt.RunOneFrame()
	rt.RunOneFrame()

	if count != 3 {
		t.Fatalf("onFrameSubmitted fired %d times, want 3", count)
	}
}

func TestOnFrameSubmitted_nil_hook_does_not_panic(t *testing.T) {
	bf := &backendFixture{}
	rt, _ := mustLifecycleRuntime(t, bf)

	rt.onFrameSubmitted = nil
	rt.RunOneFrame()
	rt.RunOneFrame()
}
