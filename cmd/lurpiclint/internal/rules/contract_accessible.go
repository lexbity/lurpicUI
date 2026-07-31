package rules

import (
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
)

// AccessibleContract (LL032) verifies that every mark implementing the
// Accessible capability — both AccessibilityRole() and AccessibleName()
// methods — has a contract test invoking contracttest.AssertAccessible.  It
// is the static mirror of the dynamic contract helper: it does not verify
// accessibility behavior; it requires the helper to be wired in so the
// behavior is proven.
//
// A mark declaring only one of the two methods is NOT an Accessible
// implementor and must not fire (partial-implementation guard).
//
// Default severity: warn (migration; ratchets to error once the backlog is
// clean).
type AccessibleContract struct{}

func (r *AccessibleContract) ID() string                     { return "LL032" }
func (r *AccessibleContract) DefaultSeverity() diag.Severity { return diag.SeverityWarn }
func (r *AccessibleContract) Description() string {
	return "mark implements the Accessible capability without a contract proof; add a test applying contracttest.AssertAccessible"
}

func (r *AccessibleContract) Explain() string {
	return `LL032: mark implements marks.Accessible but no test calls contracttest.AssertAccessible.

The Accessible capability requires BOTH AccessibilityRole() string and
AccessibleName() string (marks/capabilities.go).  This rule requires that the
mark's package contains a _test.go file that invokes contracttest.AssertAccessible
— the dynamic helper verifies the reported role is non-empty and matches the
expected canonical role, and that AccessibleName() reflects the bound label.

A mark that declares only one of the two methods is not an Accessible
implementor and is intentionally not flagged (partial-implementation guard).

Known limitation: this rule detects directly-declared AccessibilityRole and
AccessibleName methods only (method-set scanning over the type's AST).  A mark
that promotes either method via an embedded type will not be recognised —
declare the methods directly, which is the framework's established convention.

Fix: add a contract test, e.g.

    func Test<Type>_contract_accessible(t *testing.T) {
        contracttest.AssertAccessible(t, build, "<expected-role>")
    }

See marks/contracttest/AssertAccessible and marks/capabilities.go for the
Accessible interface.`
}

func (r *AccessibleContract) Check(ctx *Context) []*diag.Diagnostic {
	return capabilityRule(ctx, r, []string{"AccessibilityRole", "AccessibleName"}, "AssertAccessible", nil)
}

func init() {
	DefaultRegistry.Register(&AccessibleContract{})
}
