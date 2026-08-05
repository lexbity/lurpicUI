package runtime

import (
	"fmt"
	"runtime/debug"
	"time"

	"codeburg.org/lexbit/lurpicui/diagnostics"
	"codeburg.org/lexbit/lurpicui/facet"
)

// poisoningSink is an optional capability a DiagnosticsHook implementer may
// add (structurally) to receive per-poison events without widening the
// DiagnosticsHook interface itself. The runtime type-asserts its diagnostics
// hook to this interface, mirroring the runtimeStateSource and
// frameLayerResolver escape-hatch pattern.
type poisoningSink interface {
	OnFacetPoisoned(report diagnostics.PoisonReport)
}

// poisonReport captures the first failure of a quarantined facet so it can be
// surfaced through diagnostics or logged exactly once.
type poisonReport struct {
	FacetID   facet.FacetID
	MarkType  string
	Role      string
	Panic     any
	Stack     string
	FirstSeen time.Time
}

// describeMark returns a best-effort mark type name for diagnostics. A richer
// descriptor exists in marks/descriptor.go but runtime/ cannot import marks
// without an import cycle, so the concrete Go type name is used. The value is
// diagnostic-only and never load-bearing.
func describeMark(f facet.FacetImpl) string {
	if f == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%T", f)
}

// guardedInvoke runs a facet callback with panic recovery. On panic it
// quarantines the facet (and its subtree) and returns; role names the callback
// kind for the diagnostic. When recovery is disabled (Config.RecoveryDisabled)
// the panic is re-raised after the report is captured so the caller sees the
// original panic with attribution.
func (rt *Runtime) guardedInvoke(id facet.FacetID, role string, fn func()) {
	if rt.isPoisoned(id) {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			rt.poison(id, role, r)
			if rt.recoveryDisabled {
				panic(r)
			}
		}
	}()
	fn()
}

// isPoisoned reports whether the facet has been quarantined. Nil-safe; returns
// false when the poison state has not been initialised.
func (rt *Runtime) isPoisoned(id facet.FacetID) bool {
	rt.poisonMu.RLock()
	defer rt.poisonMu.RUnlock()
	if rt.poisoned == nil {
		return false
	}
	_, ok := rt.poisoned[id]
	return ok
}

// PoisonedCount returns the number of distinct facets currently quarantined.
// It is surfaced through diagnostics.FrameStats so a monitoring hook or test
// can detect non-zero poison without parsing logs.
func (rt *Runtime) PoisonedCount() int {
	rt.poisonMu.RLock()
	defer rt.poisonMu.RUnlock()
	return len(rt.poisoned)
}

// poison quarantines a facet after a callback panic. It is idempotent: only
// the first poisoning of a given facet is reported and logged; the panic value
// and stack are captured once and subsequent poisonings of the same facet are
// suppressed. The entire subtree rooted at the facet is quarantined so a
// half-laid-out subtree is not walked on later frames. The mark type name for
// the diagnostic is resolved lazily from the tree, so the recovery hot path
// never pays for type-name formatting.
func (rt *Runtime) poison(id facet.FacetID, role string, r any) {
	rt.poisonMu.Lock()
	defer rt.poisonMu.Unlock()
	if _, exists := rt.poisoned[id]; exists {
		return
	}
	rt.poisoned[id] = struct{}{}
	report := &poisonReport{
		FacetID:   id,
		Role:      role,
		Panic:     r,
		Stack:     rt.captureStack(4),
		FirstSeen: time.Now(),
	}
	rt.poisonReports[id] = report
	// Subtree quarantine: mark all active descendants poisoned without
	// re-reporting them.
	if root := rt.findFacetByID(rt.root, id); root != nil {
		report.MarkType = describeMark(root)
		rt.walkActive(root, func(f facet.FacetImpl) {
			if f.Base().ID() != id {
				rt.poisoned[f.Base().ID()] = struct{}{}
			}
		})
	}
	rt.log.Warn("facet quarantined after panic",
		"facetID", id, "markType", report.MarkType, "role", role, "panic", fmt.Sprint(r))
	if diag := rt.diagnosticsHook(); diag != nil {
		if sink, ok := diag.(poisoningSink); ok {
			sink.OnFacetPoisoned(diagnostics.PoisonReport{
				FacetID:   report.FacetID,
				MarkType:  report.MarkType,
				Role:      report.Role,
				Panic:     fmt.Sprint(report.Panic),
				Stack:     report.Stack,
				FirstSeen: report.FirstSeen,
			})
		}
	}
}

// captureStack returns the current goroutine stack. The skip parameter is
// retained for call-site symmetry; runtime/debug.Stack always returns the full
// current-goroutine stack.
func (rt *Runtime) captureStack(skip int) string {
	return string(debug.Stack())
}

// GuardedProject runs a projection callback under facet recovery. It returns
// true when fn ran to completion, false when the facet was already quarantined
// or was quarantined by the call itself. Projection reaches this through the
// type-asserted facetRecovery interface, so the projection callback family is
// recovered without widening facet.RuntimeServices.
func (rt *Runtime) GuardedProject(id facet.FacetID, role string, fn func()) bool {
	if rt.isPoisoned(id) {
		return false
	}
	rt.guardedInvoke(id, role, fn)
	// guardedInvoke re-panics in RecoveryDisabled mode, so reaching this point
	// means fn either ran to completion or was recovered (and the facet is now
	// quarantined).
	return !rt.isPoisoned(id)
}

// IsPoisoned reports whether a facet has been quarantined. Exposed to
// projection through the facetRecovery interface.
func (rt *Runtime) IsPoisoned(id facet.FacetID) bool {
	return rt.isPoisoned(id)
}
