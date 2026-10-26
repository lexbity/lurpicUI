package runtime

import (
	"testing"
	"time"
)

func TestWallRuntimeParksAfterBootstrapUntilFrameRequest(t *testing.T) {
	rt := mustRuntime(t)
	backend := rt.renderPipeline.backend.(*backendFixture)
	done := make(chan error, 1)
	go func() { done <- rt.Run() }()
	deadline := time.Now().Add(time.Second)
	for backend.submitCount.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if backend.submitCount.Load() < 2 {
		t.Fatalf("bootstrap and dirty follow-up frames submitted = %d, want at least 2", backend.submitCount.Load())
	}
	settled := backend.submitCount.Load()
	time.Sleep(75 * time.Millisecond)
	if got := backend.submitCount.Load(); got != settled {
		t.Fatalf("idle runtime submissions changed from %d to %d", settled, got)
	}

	rt.RequestFrame()
	deadline = time.Now().Add(time.Second)
	for backend.submitCount.Load() <= settled && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := backend.submitCount.Load(); got <= settled {
		t.Fatalf("explicit frame request did not wake idle runtime: baseline=%d current=%d", settled, got)
	}
	rt.Shutdown()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runtime did not stop after shutdown")
	}
}
