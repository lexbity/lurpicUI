package rules

import (
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
)

// FocusableContract (LL033) verifies that every mark implementing the
// Focusable capability — a Focusable() bool method — has a contract test
// invoking contracttest.AssertFocusable.  It is the static mirror of the
// dynamic contract helper: it does not verify focus behavior; it requires the
// helper to be wired in so the behavior is proven.
//
// Only directly-declared Focusable() methods on the mark type are recognised;
// a Focusable() promoted from an embedded helper struct is not the capability
// and must not fire (false-positive guard).
//
// Default severity: warn (migration; ratchets to error once the backlog is
// clean).
type FocusableContract struct{}

func (r *FocusableContract) ID() string                     { return "LL033" }
func (r *FocusableContract) DefaultSeverity() diag.Severity { return diag.SeverityWarn }
func (r *FocusableContract) Description() string {
	return "mark implements the Focusable capability without a contract proof; add a test applying contracttest.AssertFocusable"
}

func (r *FocusableContract) Explain() string {
	return `LL033: mark implements marks.Focusable but no test calls contracttest.AssertFocusable.

The Focusable capability (Focusable() bool, marks/capabilities.go) is proven
by contracttest.AssertFocusable: Focusable() returns true when enabled, false
when disabled, and a focus round-trip raises DirtyProjection when a FocusRole
is configured.

Only a Focusable() method declared directly on the mark type counts.  A
Focusable() promoted from an embedded helper struct is NOT the capability and
is intentionally not flagged — this keeps the false-positive guard narrow
(declare the method directly, which is the framework's established
convention).

Fix: add a contract test, e.g.

    func Test<Type>_contract_focusable(t *testing.T) {
        contracttest.AssertFocusable(t, build)
    }

See marks/contracttest/AssertFocusable and marks/capabilities.go for the
Focusable interface.`
}

func (r *FocusableContract) Check(ctx *Context) []*diag.Diagnostic {
	return capabilityRule(ctx, r, []string{"Focusable"}, "AssertFocusable", nil)
}

func init() {
	DefaultRegistry.Register(&FocusableContract{})
}
