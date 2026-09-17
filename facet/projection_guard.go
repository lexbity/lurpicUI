package facet

// Projection phase guard (RX-1 P5).
//
// Projection callbacks — ProjectionRole.Project and RenderRole.Collect — are
// only legal inside the runtime's single collect→project→compose phase. The
// runtime exposes a per-runtime phase check (AssertProjectionActive); a mark
// invoking projection from a store handler, a tick, or any other out-of-phase
// context is a frame-pipeline violation and must fail loudly instead of
// emitting commands at a random point in the frame.
//
// Project is guarded through the runtime carried by the ProjectionContext: a
// context with a runtime that asserts its phase is guarded, while a
// stub/isolation context (mark unit tests projecting against a fake runtime)
// is not under the frame pipeline and is left unguarded. Collect receives no
// runtime context, so it consults a package-level check installed by the
// runtime; the only Collect invocations in the framework run inside the
// projection phase.

// projectionAssertor is implemented by the runtime to report whether the
// projection phase is currently executing.
type projectionAssertor interface {
	AssertProjectionActive()
}

// assertProjectionActiveFor guards a ProjectionRole.Project invocation against
// the runtime supplied in the context. Contexts without an asserting runtime
// are not under the frame pipeline (isolation tests) and pass through.
func assertProjectionActiveFor(ctx ProjectionContext) {
	if a, ok := ctx.Runtime.(projectionAssertor); ok {
		a.AssertProjectionActive()
	}
}

// collectPhaseFunc reports whether the runtime's collect phase is executing.
// Set by the runtime at startup; nil when no runtime is live (isolation tests).
var collectPhaseFunc func() bool

// SetCollectPhaseCheck installs the function reporting whether the collect
// phase is executing. Called once at runtime startup.
func SetCollectPhaseCheck(fn func() bool) {
	collectPhaseFunc = fn
}

// assertCollectActive panics if a RenderRole.Collect invocation happens outside
// the collect→project→compose phase of a live runtime.
func assertCollectActive() {
	if collectPhaseFunc != nil && !collectPhaseFunc() {
		panic("facet: RenderRole.Collect called outside the projection phase — frame pipeline order (RX-1 P5); collect only from OnCollect")
	}
}
