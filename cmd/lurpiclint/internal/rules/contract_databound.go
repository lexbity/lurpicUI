package rules

import (
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
)

// DataBoundContract (LL029) verifies that every mark declaring the DataBound
// capability — a BoundData() any method, e.g. marks/data.DataMark — has a
// contract test invoking contracttest.AssertDataBound.  It is the static
// mirror of the dynamic contract helper: it does not verify bound-data
// behavior; it requires the helper to be wired in so the behavior is proven.
//
// Default severity: warn (migration; ratchets to error once the backlog is
// clean).
type DataBoundContract struct{}

func (r *DataBoundContract) ID() string                     { return "LL029" }
func (r *DataBoundContract) DefaultSeverity() diag.Severity { return diag.SeverityWarn }
func (r *DataBoundContract) Description() string {
	return "mark declares the DataBound capability without a contract proof; add a test applying contracttest.AssertDataBound"
}

func (r *DataBoundContract) Explain() string {
	return `LL029: mark declares the DataBound capability but no test calls contracttest.AssertDataBound.

The DataBound capability (BoundData() any) is the contract behind
marks/data.DataMark: a mark that exposes a bound-data store must prove the
round-trip survives the layout lifecycle.  This rule requires that the mark's
package contains a _test.go file that invokes contracttest.AssertDataBound —
the dynamic helper verifies the contract at runtime.

Known limitation: this rule detects directly-declared BoundData methods only
(method-set scanning over the type's AST).  A mark that promotes BoundData via
an embedded type will not be recognised — declare the method directly, which
is the framework's established convention.

Fix: add a contract test, e.g.

    func Test<Type>_contract_databound(t *testing.T) {
        contracttest.AssertDataBound[<Item>](t, build, mark, select)
    }

See marks/data/datamark_test.go for the established pattern and
marks/capabilities.go for the DataBound interface.`
}

func (r *DataBoundContract) Check(ctx *Context) []*diag.Diagnostic {
	return capabilityRule(ctx, r, []string{"BoundData"}, "AssertDataBound", nil)
}

func init() {
	DefaultRegistry.Register(&DataBoundContract{})
}
