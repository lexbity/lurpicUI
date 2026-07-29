package rules

import (
	"go/ast"
	"go/token"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/walk"
)

// LayoutCallbackAccess flags direct invocation of OnMeasure/OnArrange callbacks
// and direct writes to ArrangedBounds/MeasuredSize outside the allowlisted
// framework packages.  The public LayoutRole.Arrange / LayoutRole.Measure
// methods are the only sanctioned drive surface; bypassing them defeats the
// cache, skips the placement check, and leaves MeasuredSize stale.
//
// Writes inside an OnArrange/OnMeasure function body (the leaf-mark pattern
// where a facet writes its OWN ArrangedBounds during arrangement) are NOT
// flagged — they are the sanctioned way for a callback to report placement.
type LayoutCallbackAccess struct{}

func (r *LayoutCallbackAccess) ID() string                     { return "LL020" }
func (r *LayoutCallbackAccess) DefaultSeverity() diag.Severity { return diag.SeverityError }
func (r *LayoutCallbackAccess) Description() string {
	return "direct LayoutRole callback invocation or field write; use the public Arrange/Measure methods instead"
}

func (r *LayoutCallbackAccess) Check(ctx *Context) []*diag.Diagnostic {
	var diags []*diag.Diagnostic

	for _, f := range ctx.Files {
		if isLayoutOrMarksPackage(f) || isRuntimePackage(f) || isGraphPackage(f) || isFacetPackage(f) {
			continue
		}

		fid := facetIdent(f.Imports)
		layoutFields := layoutRoleFieldNames(f.AST, fid)

		diags = append(diags, r.checkForbiddenCalls(f, layoutFields)...)
		diags = append(diags, r.checkForbiddenWrites(f)...)
	}

	return diags
}

// checkForbiddenCalls flags direct invocations of OnMeasure or OnArrange where
// the receiver is a layout-role field (shape 1) or a LayoutRole() accessor
// result (shape 2).
func (r *LayoutCallbackAccess) checkForbiddenCalls(f *loader.ParsedFile, layoutFields map[string]bool) []*diag.Diagnostic {
	var diags []*diag.Diagnostic

	ast.Inspect(f.AST, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if sel.Sel.Name != StrOnMeasure && sel.Sel.Name != StrOnArrange {
			return true
		}

		// Shape 2: ...LayoutRole().OnX(...)
		if walk.SelectorChainContains(sel.X, "LayoutRole") {
			diags = append(diags, r.callDiagnostic(f.Fset.Position(call.Pos()), sel.Sel.Name))
			return true
		}

		// Shape 1: recv.<layoutField>.OnX(...)
		if inner, ok := sel.X.(*ast.SelectorExpr); ok && layoutFields[inner.Sel.Name] {
			diags = append(diags, r.callDiagnostic(f.Fset.Position(call.Pos()), sel.Sel.Name))
			return true
		}

		return true
	})

	return diags
}

// checkForbiddenWrites flags direct assignments to ArrangedBounds/MeasuredSize
// EXCEPT when the write is inside the body of an OnArrange/OnMeasure callback
// definition — the leaf-mark pattern where a facet writes its OWN bounds.
//
// Detection: build a set of FuncLit nodes that serve as OnArrange/OnMeasure
// callbacks; skip writes inside any of those bodies.
func (r *LayoutCallbackAccess) checkForbiddenWrites(f *loader.ParsedFile) []*diag.Diagnostic {
	var diags []*diag.Diagnostic
	forbiddenFields := []string{StrArrangedBounds, StrMeasuredSize}

	// Collect all FuncLit nodes that are assigned as OnArrange/OnMeasure callbacks.
	callbackBodies := make(map[*ast.BlockStmt]bool)
	ast.Inspect(f.AST, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.KeyValueExpr:
			if id, ok := n.Key.(*ast.Ident); ok {
				if id.Name == StrOnArrange || id.Name == StrOnMeasure {
					if fn, ok := n.Value.(*ast.FuncLit); ok && fn.Body != nil {
						callbackBodies[fn.Body] = true
					}
				}
			}
		case *ast.AssignStmt:
			for i, rhs := range n.Rhs {
				if fn, ok := rhs.(*ast.FuncLit); ok && fn.Body != nil && i < len(n.Lhs) {
					if sel, ok := n.Lhs[i].(*ast.SelectorExpr); ok {
						if sel.Sel.Name == StrOnArrange || sel.Sel.Name == StrOnMeasure {
							callbackBodies[fn.Body] = true
						}
					}
				}
			}
		}
		return true
	})

	isInCallbackBody := func(node ast.Node) bool {
		for body := range callbackBodies {
			if body.Pos() <= node.Pos() && node.End() <= body.End() {
				return true
			}
		}
		return false
	}

	// Direct assignments.
	ast.Inspect(f.AST, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for _, lhs := range assign.Lhs {
			for _, fld := range forbiddenFields {
				if walk.SelectorChainContains(lhs, fld) {
					if !isInCallbackBody(assign) {
						diags = append(diags, r.writeDiagnostic(f.Fset.Position(lhs.Pos()), fld))
					}
				}
			}
		}
		return true
	})

	// Address-of expressions.
	ast.Inspect(f.AST, func(n ast.Node) bool {
		un, ok := n.(*ast.UnaryExpr)
		if !ok || un.Op != token.AND {
			return true
		}
		for _, fld := range forbiddenFields {
			if walk.SelectorChainContains(un.X, fld) {
				if !isInCallbackBody(un) {
					diags = append(diags, r.writeDiagnostic(f.Fset.Position(un.Pos()), fld))
				}
			}
		}
		return true
	})

	return diags
}

func (r *LayoutCallbackAccess) callDiagnostic(pos token.Position, callback string) *diag.Diagnostic {
	return &diag.Diagnostic{
		RuleID:   r.ID(),
		Severity: r.DefaultSeverity(),
		Pos:      pos,
		Message:  "direct " + callback + " invocation bypasses LayoutRole.Arrange; call role.Arrange(ctx, bounds) instead",
		Teach: diag.Teaching{
			Did:      "invoked a LayoutRole callback (" + callback + ") directly",
			UseThis:  "the public role.Measure / role.Arrange methods",
			IndexRef: "facet.LayoutRole.Arrange",
		},
	}
}

func (r *LayoutCallbackAccess) writeDiagnostic(pos token.Position, field string) *diag.Diagnostic {
	return &diag.Diagnostic{
		RuleID:   r.ID(),
		Severity: r.DefaultSeverity(),
		Pos:      pos,
		Message:  "direct " + field + " assignment bypasses LayoutRole.Arrange; call role.Arrange(ctx, bounds) instead",
		Teach: diag.Teaching{
			Did:      "wrote a LayoutRole field (" + field + ") directly",
			UseThis:  "the public role.Measure / role.Arrange methods",
			IndexRef: "facet.LayoutRole.Arrange",
		},
	}
}

func init() {
	DefaultRegistry.Register(&LayoutCallbackAccess{})
}
