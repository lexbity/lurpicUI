package runtime

func (rt *Runtime) deliverSignals() {
	const maxIterations = 16
	for i := 0; ; i++ {
		if len(rt.signalQueue) == 0 {
			return
		}
		if i >= maxIterations {
			panic("runtime: signal delivery exceeded 16 iterations in one frame — likely a signal cycle; check store mutation inside signal handlers")
		}
		batch := append([]pendingSignal(nil), rt.signalQueue...)
		for j := range rt.signalQueue {
			rt.signalQueue[j] = pendingSignal{}
		}
		rt.signalQueue = rt.signalQueue[:0]
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
	if runtimeTraceActive() {
		runtimeTracef("queueSignal pending=%d", len(rt.signalQueue)+1)
	}
	rt.signalQueue = append(rt.signalQueue, pendingSignal{deliver: deliver})
}
