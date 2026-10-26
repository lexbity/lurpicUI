package runtime

// deliverSignals drains the queued store notifications. It runs on the runtime
// thread once per frame, before layout and projection.
//
// signalMu guards the queue itself: the projection system forks goroutines
// (projection.go walkChildSubtrees) that can enqueue signals concurrently —
// a Derived recompute during a forked projection reads a cold cache and emits
// OnChange via store.enqueueSignal → rt.queueSignal. Without the guard, two
// forked projection goroutines appending to rt.signalQueue race
// (F-signal-queue-race). The frame loop serializes deliverSignals against the
// forked goroutines (they are joined by wg.Wait inside the frame), so the lock
// is only contended between the forked goroutines themselves.
func (rt *Runtime) deliverSignals() {
	rt.config.StoreScope.FlushSignals()
}
