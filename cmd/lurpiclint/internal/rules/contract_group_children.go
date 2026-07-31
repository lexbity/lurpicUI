package rules

import (
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/capindex"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
)

// GroupChildrenContract (LL031) verifies that every container mark exposing
// children via a Children() []facet.GroupChild method has a contract test
// invoking contracttest.AssertGroupChildren.  It is the static mirror of the
// dynamic contract helper: it does not verify group-child behavior; it
// requires the helper to be wired in so the behavior is proven.
//
// Unlike LL029/LL030 this rule gates on the capability fingerprint first:
// only marks classified as containers (capindex.Fingerprint.IsContainer) are
// in scope, and a container mark without a directly-declared Children()
// method is a leaf with a container-shaped fingerprint — it must NOT fire
// (R-3 over-fire guard).
//
// Default severity: error — lurpiclint check ./... --fail-on error blocks any
// future capability-contract omission.
type GroupChildrenContract struct{}

func (r *GroupChildrenContract) ID() string                     { return "LL031" }
func (r *GroupChildrenContract) DefaultSeverity() diag.Severity { return diag.SeverityError }
func (r *GroupChildrenContract) Description() string {
	return "container mark exposes Children() []facet.GroupChild without a contract proof; add a test applying contracttest.AssertGroupChildren"
}

func (r *GroupChildrenContract) Explain() string {
	return `LL031: container mark exposes Children() []facet.GroupChild but no test calls contracttest.AssertGroupChildren.

The group-children capability (a Children() []facet.GroupChild method on a
container mark such as marks/structure.Card or marks/structure.List) is proven
by contracttest.AssertGroupChildren: child list non-empty, every entry's
Layout non-nil, every facet-registered child represented, disposal propagates,
and a rebuild reproduces the child count.

This rule only fires on marks classified as containers by the capability
fingerprint (embeds facet.Facet and registers a layout role) AND that declare
a Children() method directly.  A container-shaped fingerprint without a
Children() method is a leaf mark and is intentionally skipped.

Known limitation: this rule detects directly-declared Children methods only
(method-set scanning over the type's AST).  A mark that promotes Children via
an embedded type will not be recognised — declare the method directly, which
is the framework's established convention.

Fix: add a contract test, e.g.

    func Test<Type>_contract_group_children(t *testing.T) {
        contracttest.AssertGroupChildren(t, build, fixDataState)
    }

See marks/contracttest/AssertGroupChildren, marks/structure/card_test.go, and
facet/layout_role.go for the GroupChild contract type.`
}

func (r *GroupChildrenContract) Check(ctx *Context) []*diag.Diagnostic {
	// R-3 over-fire guard: only marks whose fingerprint classifies them as
	// containers AND that declare a Children() method directly are in scope.
	// A container fingerprint without a Children() method is a leaf; a
	// Children() method without a container fingerprint is not a group
	// source — both are skipped rather than demanding a contract.
	gate := func(c capindex.Capability) bool {
		return c.Fingerprint.IsContainer && packageHasMethod(ctx, c, "Children")
	}
	return capabilityRule(ctx, r, []string{"Children"}, "AssertGroupChildren", gate)
}

func init() {
	DefaultRegistry.Register(&GroupChildrenContract{})
}
