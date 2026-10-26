package runtime

import (
	"testing"
	"time"
)

func TestPhase1HookReleaseAndReuseAreGenerationSafe(t *testing.T) {
	rt := &Runtime{}
	firstCalls := 0
	releaseFirst := rt.RegisterPhase1TickHook(func(time.Duration) { firstCalls++ })
	releaseFirst()
	releaseFirst()
	secondCalls := 0
	secondRelease := rt.RegisterPhase1TickHook(func(time.Duration) { secondCalls++ })
	rt.runPhase1TickHooks(time.Millisecond)
	if firstCalls != 0 || secondCalls != 1 {
		t.Fatalf("hook calls after slot reuse: first=%d second=%d", firstCalls, secondCalls)
	}
	secondRelease()
	rt.runPhase1TickHooks(time.Millisecond)
	if secondCalls != 1 {
		t.Fatalf("released hook ran again: calls=%d", secondCalls)
	}
	if len(rt.phase1Hooks) != 1 || rt.phase1Hooks[0].fn != nil || len(rt.phase1HookFree) != 1 {
		t.Fatalf("hook storage did not reuse its slot: slots=%d free=%v", len(rt.phase1Hooks), rt.phase1HookFree)
	}
}

func TestPhase1HookRegisteredDuringDispatchStartsNextTurn(t *testing.T) {
	rt := &Runtime{}
	firstCalls, laterCalls := 0, 0
	var releaseLater func()
	rt.RegisterPhase1TickHook(func(time.Duration) {
		firstCalls++
		if releaseLater == nil {
			releaseLater = rt.RegisterPhase1TickHook(func(time.Duration) { laterCalls++ })
		}
	})
	rt.runPhase1TickHooks(time.Millisecond)
	if firstCalls != 1 || laterCalls != 0 {
		t.Fatalf("first dispatch calls: first=%d later=%d", firstCalls, laterCalls)
	}
	rt.runPhase1TickHooks(time.Millisecond)
	if firstCalls != 2 || laterCalls != 1 {
		t.Fatalf("second dispatch calls: first=%d later=%d", firstCalls, laterCalls)
	}
	releaseLater()
}

func TestPhase1HookReleaseCompactsActiveEntries(t *testing.T) {
	rt := &Runtime{}
	calls := 0
	for i := 0; i < 128; i++ {
		release := rt.RegisterPhase1TickHook(func(time.Duration) { calls++ })
		release()
	}
	wantRelease := rt.RegisterPhase1TickHook(func(time.Duration) { calls++ })
	rt.runPhase1TickHooks(0)
	if calls != 1 {
		t.Fatalf("active callbacks = %d, want 1", calls)
	}
	if got := len(rt.phase1HookActive); got != 1 {
		t.Fatalf("active hook directory length = %d, want 1", got)
	}
	if got := len(rt.phase1HookFree); got != 0 {
		t.Fatalf("reusable hook slots = %d, want 0", got)
	}
	wantRelease()
	if got := len(rt.phase1HookActive); got != 0 {
		t.Fatalf("active hook directory after release = %d, want 0", got)
	}
	if got := len(rt.phase1HookFree); got != 1 {
		t.Fatalf("reusable hook slots after release = %d, want 1", got)
	}
}
