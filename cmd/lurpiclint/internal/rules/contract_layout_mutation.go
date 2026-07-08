package rules

import (
	"go/ast"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/walk"
)

// LayoutCallbackStoreMutation flags store mutations inside OnMeasure or
// OnArrange function literals assigned to a facet.LayoutRole.  Layout
// callbacks must be read-only (Runtime Principles 1 and 8).
//
// Detection uses a method-name heuristic:
//   - Allowlisted (read-only): Get, Version, All, Identify, Len, Length
//   - Flagged (mutating): Set, Insert, Update, Remove, Delete
//
// Default severity: error.
type LayoutCallbackStoreMutation struct{}

func (r *LayoutCallbackStoreMutation) ID() string                     { return "LL016" }
func (r *LayoutCallbackStoreMutation) DefaultSeverity() diag.Severity { return diag.SeverityError }
func (r *LayoutCallbackStoreMutation) Description() string {
	return "store mutation in OnMeasure or OnArrange callback; layout callbacks must be read-only (Principles 1 and 8)"
}

func (r *LayoutCallbackStoreMutation) Check(ctx *Context) []*diag.Diagnostic {
	var diags []*diag.Diagnostic

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

			if kv := walk.KeyValue(lit, "OnMeasure"); kv != nil {
				if fn, ok := kv.(*ast.FuncLit); ok {
					diags = append(diags, r.checkFunctionBody(fn, f, "OnMeasure")...)
				}
			}

			if kv := walk.KeyValue(lit, "OnArrange"); kv != nil {
				if fn, ok := kv.(*ast.FuncLit); ok {
					diags = append(diags, r.checkFunctionBody(fn, f, "OnArrange")...)
				}
			}

			return true
		})
	}

	return diags
}

func (r *LayoutCallbackStoreMutation) checkFunctionBody(fn *ast.FuncLit, f *loader.ParsedFile, callbackName string) []*diag.Diagnostic {
	var diags []*diag.Diagnostic

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		if !r.isMutatingStoreCall(call) {
			return true
		}

		diags = append(diags, &diag.Diagnostic{
			RuleID:   r.ID(),
			Severity: r.DefaultSeverity(),
			Pos:      f.Fset.Position(call.Pos()),
			Message:  "store mutation in " + callbackName + " callback; layout callbacks must be read-only",
			Teach: diag.Teaching{
				Did:      "called a mutating store method in layout callback",
				UseThis:  "move state mutations outside layout callbacks or use read-only methods",
				IndexRef: "",
			},
		})

		return true
	})

	return diags
}

// isMutatingStoreCall returns true when call is a selector expression whose
// method name matches a known store-mutation pattern and is not on the
// read-only allowlist.
func (r *LayoutCallbackStoreMutation) isMutatingStoreCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	methodName := sel.Sel.Name

	// Read-only store methods — always allowed.
	readOnly := map[string]bool{
		"Get":      true,
		"Version":  true,
		"All":      true,
		"Identify": true,
		"Len":      true,
		"Length":   true,
	}
	if readOnly[methodName] {
		return false
	}

	// Mutating store methods — always flagged.
	mutating := map[string]bool{
		"Set":    true,
		"Insert": true,
		"Update": true,
		"Remove": true,
		"Delete": true,
	}

	return mutating[methodName]
}

func init() {
	DefaultRegistry.Register(&LayoutCallbackStoreMutation{})
}
