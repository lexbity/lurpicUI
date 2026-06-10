package rules

import (
	"go/ast"
	"go/token"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/walk"
)

// FacetGoroutine flags raw goroutines and channel operations inside types
// that embed facet.Facet.  Runtime Principle 5 requires facets to be
// single-threaded; use job.Schedule for deferred work instead.
//
// Default severity: error.
type FacetGoroutine struct{}

func (r *FacetGoroutine) ID() string                     { return "LL011" }
func (r *FacetGoroutine) DefaultSeverity() diag.Severity { return diag.SeverityError }
func (r *FacetGoroutine) Description() string {
	return "goroutine or channel operation in facet code; use job.Schedule instead"
}

func (r *FacetGoroutine) Check(ctx *Context) []*diag.Diagnostic {
	var diags []*diag.Diagnostic

	for _, f := range ctx.Files {
		// Skip files that don't define any facet-embedding types.
		if !fileContainsFacetType(f) {
			continue
		}

		// Scan for goroutines, channel ops, and channel types.
		ast.Inspect(f.AST, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.GoStmt:
				// Allowlist: go calls to job.Schedule are permitted.
				if isJobScheduleCall(n.Call) {
					return true
				}
				diags = append(diags, r.diag(f, n.Pos(), "raw goroutine: use job.Schedule instead"))
				return true

			case *ast.CallExpr:
				// Allowlisted patterns.
				if isJobScheduleCall(n) {
					return true
				}
				return true

			case *ast.SendStmt:
				diags = append(diags, r.diag(f, n.Pos(), "channel send operation in facet code"))
				return true

			case *ast.UnaryExpr:
				if n.Op == token.ARROW {
					diags = append(diags, r.diag(f, n.Pos(), "channel receive operation in facet code"))
				}
				return true

			case *ast.ChanType:
				diags = append(diags, r.diag(f, n.Pos(), "channel type in facet code"))
				return true
			}
			return true
		})
	}

	return diags
}

func (r *FacetGoroutine) diag(f *loader.ParsedFile, pos token.Pos, msg string) *diag.Diagnostic {
	return &diag.Diagnostic{
		RuleID:   r.ID(),
		Severity: r.DefaultSeverity(),
		Pos:      f.Fset.Position(pos),
		Message:  msg,
		Teach: diag.Teaching{
			Did:      "used a goroutine or channel in facet code",
			UseThis:  "job.Schedule for deferred work, or coordinate via the facet lifecycle",
			IndexRef: "",
		},
	}
}

// isJobScheduleCall returns true if call is a function call to job.Schedule.
func isJobScheduleCall(call *ast.CallExpr) bool {
	if call == nil {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return id.Name == "job" && sel.Sel.Name == "Schedule"
}

// fileContainsFacetType reports whether any type declaration in the file
// embeds facet.Facet.
func fileContainsFacetType(f *loader.ParsedFile) bool {
	for _, decl := range f.AST.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if walk.EmbedsFacet(ts, f.Imports) {
				return true
			}
		}
	}
	return false
}

func init() {
	DefaultRegistry.Register(&FacetGoroutine{})
}
