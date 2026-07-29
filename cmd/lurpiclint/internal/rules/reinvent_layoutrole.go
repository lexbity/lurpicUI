package rules

import (
	"go/ast"
	"go/token"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/walk"
)

// ReinventLayoutRole flags direct population of a facet.LayoutRole's
// OnMeasure or OnArrange callback outside the layout/ or marks/ packages.
//
// Two assignment shapes trigger LL001:
//
//  1. Composite literal:   facet.LayoutRole{ OnMeasure: func(...) {...} }
//  2. Field assignment:    r.layout.OnMeasure = func(...) {...}
//     b.layout.OnArrange  = func(...) {...}
//
// Both shapes bypass the framework's layout/mark encapsulation.  Field
// assignment to a struct field of type facet.LayoutRole must be detected
// separately because the composite-literal scan cannot see it, and that is
// exactly the pattern agentic tooling tends to reinvent.
//
// Default severity: warn (leaf marks are not penalised; the hard gate is
// LL003 which fires only when the role arranges children).
type ReinventLayoutRole struct{}

func (r *ReinventLayoutRole) ID() string                     { return "LL001" }
func (r *ReinventLayoutRole) DefaultSeverity() diag.Severity { return diag.SeverityWarn }
func (r *ReinventLayoutRole) Description() string {
	return "raw LayoutRole with OnMeasure or OnArrange set outside layout/ or marks/ (prefer composition)"
}

func (r *ReinventLayoutRole) Check(ctx *Context) []*diag.Diagnostic {
	var diags []*diag.Diagnostic

	for _, f := range ctx.Files {
		if isLayoutOrMarksPackage(f) || isGraphPackage(f) {
			continue
		}
		fid := facetIdent(f.Imports)

		diags = append(diags, r.checkCompositeLiterals(f, fid)...)
		diags = append(diags, r.checkFieldAssignments(f, fid)...)
	}

	return diags
}

// checkCompositeLiterals returns the original LL001 findings: non-empty
// facet.LayoutRole{OnMeasure: ...} / {OnArrange: ...} literals.
func (r *ReinventLayoutRole) checkCompositeLiterals(f *loader.ParsedFile, fid string) []*diag.Diagnostic {
	var diags []*diag.Diagnostic

	ast.Inspect(f.AST, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		if !walk.CompositeLitIs(lit, fid, "LayoutRole") {
			return true
		}

		hasOnMeasure := walk.KeyValue(lit, StrOnMeasure) != nil
		hasOnArrange := walk.KeyValue(lit, StrOnArrange) != nil
		if !hasOnMeasure && !hasOnArrange {
			return true
		}

		diags = append(diags, r.diagnosticAt(
			f.Fset.Position(lit.Pos()),
			"raw LayoutRole literal with OnMeasure or OnArrange set; prefer using an existing layout container or mark",
			"populated a LayoutRole struct directly",
		))
		return true
	})

	return diags
}

// checkFieldAssignments flags assignments of the form
//
//	<receiver>.<field>.OnMeasure = func(...) {...}
//	<receiver>.<field>.OnArrange  = func(...) {...}
//
// where <field> is a struct field whose declared type is facet.LayoutRole.
// Receiver is the receiver ident of a method, a local, or a chained
// selector; the field name is matched against the file's set of
// LayoutRole-typed struct field names so unrelated .OnMeasure assignments
// (rare outside this rule) are skipped.
func (r *ReinventLayoutRole) checkFieldAssignments(f *loader.ParsedFile, fid string) []*diag.Diagnostic {
	var diags []*diag.Diagnostic

	layoutFields := layoutRoleFieldNames(f.AST, fid)
	if len(layoutFields) == 0 {
		return diags
	}

	cbs := map[string]bool{StrOnMeasure: true, StrOnArrange: true}

	ast.Inspect(f.AST, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		if len(assign.Lhs) != len(assign.Rhs) {
			return true
		}
		for i := range assign.Lhs {
			if !r.isLayoutRoleCallbackAssign(assign.Lhs[i], assign.Rhs[i], layoutFields, cbs) {
				continue
			}
			diags = append(diags, r.diagnosticAt(
				f.Fset.Position(assign.Lhs[i].Pos()),
				"facet.LayoutRole OnMeasure/OnArrange assigned outside layout/ or marks/; prefer using an existing layout container or mark",
				"set LayoutRole callbacks directly outside the layout/ or marks/ packages",
			))
		}
		return true
	})

	return diags
}

// isLayoutRoleCallbackAssign reports whether lhs is <X>.<field>.<callback>
// assigned a function literal, where <field> is one of the file's
// LayoutRole-typed struct field names and <callback> is OnMeasure or
// OnArrange.
func (r *ReinventLayoutRole) isLayoutRoleCallbackAssign(lhs, rhs ast.Expr, layoutFields map[string]bool, cbs map[string]bool) bool {
	sel, ok := lhs.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	if !cbs[sel.Sel.Name] {
		return false
	}
	inner, ok := sel.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	if !layoutFields[inner.Sel.Name] {
		return false
	}
	_, ok = rhs.(*ast.FuncLit)
	return ok
}

func (r *ReinventLayoutRole) diagnosticAt(pos token.Position, message, did string) *diag.Diagnostic {
	return &diag.Diagnostic{
		RuleID:   r.ID(),
		Severity: r.DefaultSeverity(),
		Pos:      pos,
		Message:  message,
		Teach: diag.Teaching{
			Did:      did,
			UseThis:  "an existing layout container such as layout.NewColumnLayout, or a marks/structure composite",
			IndexRef: StrLayoutColumn,
		},
	}
}

func init() {
	DefaultRegistry.Register(&ReinventLayoutRole{})
}
