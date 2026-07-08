package rules

import (
	"go/ast"
	"strings"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/capindex"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/classify"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/walk"
)

// SuggestShapeMatch emits info-level suggestions when a child-arranging
// LayoutRole's structural fingerprint matches a known capability (mark or
// layout container) in the framework's capability index.
//
// Default severity: info (suggestions, not violations).
type SuggestShapeMatch struct{}

func (r *SuggestShapeMatch) ID() string                     { return "LL004" }
func (r *SuggestShapeMatch) DefaultSeverity() diag.Severity { return diag.SeverityInfo }
func (r *SuggestShapeMatch) Description() string {
	return "child-arranging facet matches a known built-in capability; consider using it directly — also flags scalar accessor closures in viz.New* calls"
}

func (r *SuggestShapeMatch) Check(ctx *Context) []*diag.Diagnostic {
	var diags []*diag.Diagnostic

	// Extension: flag scalar accessor closures in viz.New* calls and
	// suggest data.Encoding for declarative specification instead.
	for _, f := range ctx.Files {
		ast.Inspect(f.AST, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if !r.isVizNewCall(call, f) {
				return true
			}

			for _, arg := range call.Args {
				fn, ok := arg.(*ast.FuncLit)
				if !ok {
					continue
				}
				if !r.isScalarAccessor(fn) {
					continue
				}

				diags = append(diags, &diag.Diagnostic{
					RuleID:   r.ID(),
					Severity: r.DefaultSeverity(),
					Pos:      f.Fset.Position(fn.Pos()),
					Message:  "scalar accessor closure passed to viz.New*; consider using data.Encoding for declarative channel specification",
					Teach: diag.Teaching{
						Did:      "passed scalar accessor closure to viz.New*",
						UseThis:  "data.Encoding with explicit channel specification",
						IndexRef: "marks/data/",
					},
				})
			}

			return true
		})
	}

	// Existing: child-arranging LayoutRole shape-matching (needs index).
	idx, ok := ctx.Index.([]capindex.Capability)
	if !ok || len(idx) == 0 {
		return diags
	}

	for _, f := range ctx.Files {
		fid := facetIdent(f.Imports)

		ast.Inspect(f.AST, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			if !walk.CompositeLitIs(lit, fid, "LayoutRole") {
				return true
			}

			// Only suggest for child-arranging LayoutRoles (LL003 territory).
			if !classify.IsChildArranging(lit, f.Fset, f.Imports) {
				return true
			}

			// Find a matching capability by fingerprint.
			match := findMatchingCapability(idx)
			if match == nil {
				return true
			}

			diags = append(diags, &diag.Diagnostic{
				RuleID:   r.ID(),
				Severity: r.DefaultSeverity(),
				Pos:      f.Fset.Position(lit.Pos()),
				Message: "this child-arranging LayoutRole resembles " + match.Path +
					"; consider using " + match.Constructor + " instead",
				Teach: diag.Teaching{
					Did:      "wrote a custom facet that arranges children",
					UseThis:  match.Path + " (" + match.Constructor + ")",
					IndexRef: match.Path,
				},
			})
			return true
		})
	}

	return diags
}

// isVizNewCall returns true when call is a viz.New* expression where "viz"
// resolves to the lurpicui marks/viz package.
func (r *SuggestShapeMatch) isVizNewCall(call *ast.CallExpr, f *loader.ParsedFile) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok || id.Name != "viz" {
		return false
	}
	if !strings.HasPrefix(sel.Sel.Name, "New") {
		return false
	}

	// Verify that "viz" resolves to marks/viz via the import table.
	imp, ok := f.Imports["viz"]
	if !ok {
		return false
	}
	return strings.HasSuffix(imp, "/viz") || imp == "viz"
}

// isScalarAccessor returns true when fn is a function literal whose return
// type is a scalar (string, float64, float32, int, int32, int64, bool) and
// whose body is a single return of a field access expression.
func (r *SuggestShapeMatch) isScalarAccessor(fn *ast.FuncLit) bool {
	if fn.Type == nil || fn.Type.Results == nil {
		return false
	}
	if len(fn.Type.Results.List) != 1 {
		return false
	}

	retType := fn.Type.Results.List[0].Type
	ident, ok := retType.(*ast.Ident)
	if !ok {
		return false
	}

	primitiveTypes := map[string]bool{
		"string":  true,
		"float64": true,
		"float32": true,
		"int":     true,
		"int32":   true,
		"int64":   true,
		"bool":    true,
	}
	if !primitiveTypes[ident.Name] {
		return false
	}

	// Body must be a single return statement.
	if len(fn.Body.List) != 1 {
		return false
	}
	retStmt, ok := fn.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(retStmt.Results) != 1 {
		return false
	}

	// Return expression must be a selector (field access).
	_, isFieldAccess := retStmt.Results[0].(*ast.SelectorExpr)
	return isFieldAccess
}

// findMatchingCapability picks a known capability that is a container (mark
// or layout).  For Phase 10 this uses a simple heuristic: prefer the first
// mark with IsContainer=true, falling back to the first layout container.
func findMatchingCapability(idx []capindex.Capability) *capindex.Capability {
	for i := range idx {
		if idx[i].Kind == capindex.KindMark && idx[i].Fingerprint.IsContainer {
			return &idx[i]
		}
	}
	for i := range idx {
		if idx[i].Kind == capindex.KindLayout {
			return &idx[i]
		}
	}
	return nil
}

func init() {
	DefaultRegistry.Register(&SuggestShapeMatch{})
}
