package runtime

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
)

func TestRuntime_signal_cycle_panics(t *testing.T) {
	rt := mustRuntime(t)
	defer rt.Shutdown()

	// start() sets the signal queue hook so enqueueSignal queues instead of
	// delivering recursively.
	if err := rt.start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	v := store.NewValueStore(0)
	v.OnChange.Subscribe(func(c signal.Change[int]) {
		if c.New == 1 {
			v.Set(2)
		} else {
			v.Set(1)
		}
	})

	// Queue a signal that creates a cycle: handler toggles the value,
	// which queues another signal, which toggles again, etc.
	rt.queueSignal(func() {
		v.Set(1)
	})

	expectPanicContains(t, "signal delivery exceeded", func() {
		rt.deliverSignals()
	})
}
