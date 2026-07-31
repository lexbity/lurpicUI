package rules

import (
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
)

// AnchorExportContract (LL030) verifies that every mark implementing the
// AnchorExporting capability — layout.AnchorExporter, i.e. an ExportAnchors
// method — has a contract test invoking contracttest.AssertAnchorExport.  It
// is the static mirror of the dynamic contract helper: it does not verify
// anchor behavior; it requires the helper to be wired in so the behavior is
// proven.
//
// Default severity: warn (migration; ratchets to error once the backlog is
// clean).
type AnchorExportContract struct{}

func (r *AnchorExportContract) ID() string                     { return "LL030" }
func (r *AnchorExportContract) DefaultSeverity() diag.Severity { return diag.SeverityWarn }
func (r *AnchorExportContract) Description() string {
	return "mark implements the AnchorExporting capability without a contract proof; add a test applying contracttest.AssertAnchorExport"
}

func (r *AnchorExportContract) Explain() string {
	return `LL030: mark implements layout.AnchorExporter but no test calls contracttest.AssertAnchorExport.

The AnchorExporting capability (ExportAnchors(ctx layout.AnchorExportContext)
layout.AnchorSet, aliased as marks.AnchorExporting) is the contract behind
marks that publish spatial anchors.  This rule requires that the mark's
package contains a _test.go file that invokes contracttest.AssertAnchorExport
— the dynamic helper verifies anchors are in-bounds, deterministic, and
rebuild-surviving at runtime.

Known limitation: this rule detects directly-declared ExportAnchors methods
only (method-set scanning over the type's AST).  A mark that promotes
ExportAnchors via an embedded type will not be recognised — declare the method
directly, which is the framework's established convention.  marks.Core's
DefaultAnchors is not an AnchorExporter implementation and is intentionally
not flagged.

Fix: add a contract test, e.g.

    func Test<Type>_contract_anchor_export(t *testing.T) {
        contracttest.AssertAnchorExport(t, build, mark)
    }

See marks/contracttest/AssertAnchorExport, marks/structure/card_test.go, and
marks/capabilities.go for the AnchorExporting alias.`
}

func (r *AnchorExportContract) Check(ctx *Context) []*diag.Diagnostic {
	return capabilityRule(ctx, r, []string{"ExportAnchors"}, "AssertAnchorExport", nil)
}

func init() {
	DefaultRegistry.Register(&AnchorExportContract{})
}
