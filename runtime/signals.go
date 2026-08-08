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
	const maxIterations = 16
	for i := 0; ; i++ {
		rt.signalMu.Lock()
		if len(rt.signalQueue) == 0 {
			rt.signalMu.Unlock()
			return
		}
		if i >= maxIterations {
			rt.signalMu.Unlock()
			panic("runtime: signal delivery exceeded 16 iterations in one frame — likely a signal cycle; check store mutation inside signal handlers")
		}
		batch := append([]pendingSignal(nil), rt.signalQueue...)
		for j := range rt.signalQueue {
			rt.signalQueue[j] = pendingSignal{}
		}
		rt.signalQueue = rt.signalQueue[:0]
		rt.signalMu.Unlock()
		for _, s := range batch {
			if s.deliver != nil {
				s.deliver()
			}
		}
	}
}

func (rt *Runtime) queueSignal(deliver func()) {
	if deliver == nil {
		return
	}
	rt.signalMu.Lock()
	defer rt.signalMu.Unlock()
	if runtimeTraceActive() {
		runtimeTracef("queueSignal pending=%d", len(rt.signalQueue)+1)
	}
	rt.signalQueue = append(rt.signalQueue, pendingSignal{deliver: deliver})
}
