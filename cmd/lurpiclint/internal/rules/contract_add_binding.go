package rules

import (
	"go/ast"
	"strings"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
)

// AddBindingExternal flags Core.AddBinding call sites outside the marks
// package tree (RX-2 FR-1 / Q1). The binding contract is structural: a mark's
// exported binding fields are declared by the struct and subscribed by Core
// at attach, so assignment before attach is the entire authoring surface.
// AddBinding survives as mark-internal machinery (bindings a mark creates for
// itself after construction); an app-side AddBinding call means the author is
// re-registering a binding field — the RX-1 A-6 trap ("reads live but never
// invalidates") that declared bindings exist to make unrepresentable.
type AddBindingExternal struct{}

func (r *AddBindingExternal) ID() string                     { return "LL037" }
func (r *AddBindingExternal) DefaultSeverity() diag.Severity { return diag.SeverityError }
func (r *AddBindingExternal) Description() string {
	return "Core.AddBinding call sites are illegal outside the marks package — assign the binding field before attach instead (RX-2 FR-1)"
}

func (r *AddBindingExternal) Explain() string {
	return `LL037: AddBinding outside the marks package.

Since RX-2 declared bindings, a mark's exported binding FIELDS are its binding
contract: Core subscribes every dynamic field at attach, so replacing a field
any time before attach just works. AddBinding is mark-internal machinery for
bindings the mark creates for itself after construction.

An app-side AddBinding call re-registers a binding field — and before RX-2
that was the only way to make a replaced field invalidate, which is why the
flagship demo carried a five-call re-registration ritual (the RX-1 A-6 trap).
Re-registering does not compose: it double-subscribes under declared bindings
and hides the field-replacement contract.

Fix: assign the binding field before the mark attaches:
    mark.Label = marks.FromStore(myStore, facet.DirtyProjection)
and delete the AddBinding call.`
}

func (r *AddBindingExternal) Check(ctx *Context) []*diag.Diagnostic {
	var diags []*diag.Diagnostic

	for _, f := range ctx.Files {
		if isLayoutOrMarksPackage(f) || isTestFile(f) {
			continue
		}
		ast.Inspect(f.AST, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "AddBinding" {
				return true
			}
			diags = append(diags, &diag.Diagnostic{
				RuleID:   r.ID(),
				Severity: r.DefaultSeverity(),
				Pos:      f.Fset.Position(call.Pos()),
				Message:  "AddBinding outside marks/ — assign the binding field before attach instead (declared bindings, RX-2 FR-1)",
				Teach: diag.Teaching{
					Did:      "re-registered a mark's binding through AddBinding",
					UseThis:  "assign the binding field before the mark attaches (mark.Field = marks.FromStore(...)); declared bindings subscribe fields at attach",
					IndexRef: "marks/binding.go content binding contract (RX-2 Q1)",
				},
			})
			return true
		})
	}
	return diags
}

// isTestFile reports whether the parsed file is a Go test file.
func isTestFile(f *loader.ParsedFile) bool {
	return strings.HasSuffix(f.Path, "_test.go")
}

func init() {
	DefaultRegistry.Register(&AddBindingExternal{})
}
