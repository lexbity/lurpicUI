package runtime

// deliverSignals drains the queued store notifications. It runs on the runtime
// thread once per frame, before layout and projection.
//
// Store notifications are delivered at the runtime's phase-6 cut. Projection
// callbacks are serial and store mutation is fenced during projection, so they
// cannot race this delivery boundary.
func (rt *Runtime) deliverSignals() {
	rt.config.StoreScope.FlushSignals()
}
