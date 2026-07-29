package rules

import (
	"go/ast"
	"strings"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
)

// OverlayMissingMount warns when overlay-type values are created inside a
// facet constructor but are not passed to AddChild or AddChildRuntime.
// Unmounted overlays may render incorrectly or miss lifecycle events.
//
// Detection is heuristic: it scans constructor function bodies for overlay
// constructor calls (e.g. feedback.NewDialog) and checks whether each
// resulting variable is referenced in an AddChild or AddChildRuntime call.
//
// Default severity: warn.
type OverlayMissingMount struct{}

func (r *OverlayMissingMount) ID() string                     { return "LL018" }
func (r *OverlayMissingMount) DefaultSeverity() diag.Severity { return diag.SeverityWarn }
func (r *OverlayMissingMount) Description() string {
	return "overlay value not mounted via AddChild or AddChildRuntime; overlay may render incorrectly"
}

func (r *OverlayMissingMount) Check(ctx *Context) []*diag.Diagnostic {
	var diags []*diag.Diagnostic

	for _, f := range ctx.Files {
		if !fileContainsFacetType(f) {
			continue
		}

		for _, decl := range f.AST.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if !r.isConstructorFunction(fn) {
				continue
			}

			overlayVars := r.findOverlayVariables(fn, f)
			if len(overlayVars) == 0 {
				continue
			}

			mountedVars := r.findMountedVariables(fn, f)

			for varName := range overlayVars {
				if !mountedVars[varName] {
					diags = append(diags, &diag.Diagnostic{
						RuleID:   r.ID(),
						Severity: r.DefaultSeverity(),
						Pos:      f.Fset.Position(fn.Pos()),
						Message:  "overlay variable " + varName + " not mounted via AddChild or AddChildRuntime",
						Teach: diag.Teaching{
							Did:      "created overlay without mounting it",
							UseThis:  "call Facet.AddChild or Facet.AddChildRuntime with the overlay's Base()",
							IndexRef: "",
						},
					})
				}
			}
		}
	}

	return diags
}

// isConstructorFunction returns true when fn is a constructor (name starts
// with "new" or "New").
func (r *OverlayMissingMount) isConstructorFunction(fn *ast.FuncDecl) bool {
	name := fn.Name.Name
	return strings.HasPrefix(name, "new") || strings.HasPrefix(name, "New")
}

// findOverlayVariables collects variable names whose value comes from an
// overlay constructor call (e.g. feedback.NewDialog).
func (r *OverlayMissingMount) findOverlayVariables(fn *ast.FuncDecl, f *loader.ParsedFile) map[string]bool {
	overlays := make(map[string]bool)

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}

		for i, lhs := range assign.Lhs {
			if i >= len(assign.Rhs) {
				continue
			}
			ident, ok := lhs.(*ast.Ident)
			if !ok {
				continue
			}
			call, ok := assign.Rhs[i].(*ast.CallExpr)
			if !ok {
				continue
			}
			if r.isOverlayConstructor(call, f) {
				overlays[ident.Name] = true
			}
		}

		return true
	})

	return overlays
}

// findMountedVariables collects variable names that are passed to AddChild
// or AddChildRuntime as .Base() or .Base method expressions.
func (r *OverlayMissingMount) findMountedVariables(fn *ast.FuncDecl, f *loader.ParsedFile) map[string]bool {
	mounted := make(map[string]bool)

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if !r.isAddChildCall(call) {
			return true
		}
		if len(call.Args) == 0 {
			return true
		}

		arg := call.Args[0]

		// Pattern 1: dialog.Base() — method call.
		if innerCall, ok := arg.(*ast.CallExpr); ok {
			if sel, ok := innerCall.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Base" {
				if ident, ok := sel.X.(*ast.Ident); ok {
					mounted[ident.Name] = true
				}
			}
			return true
		}

		// Pattern 2: dialog.Base — method value (selector).
		if sel, ok := arg.(*ast.SelectorExpr); ok && sel.Sel.Name == "Base" {
			if ident, ok := sel.X.(*ast.Ident); ok {
				mounted[ident.Name] = true
			}
		}

		return true
	})

	return mounted
}

// isOverlayConstructor returns true when call is an overlay constructor like
// feedback.NewDialog, navigation.NewNavDrawer, etc.
func (r *OverlayMissingMount) isOverlayConstructor(call *ast.CallExpr, f *loader.ParsedFile) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}

	// Verify the package name resolves to a known overlay package via the
	// import table, handling aliased imports correctly.
	imp, ok := f.Imports[id.Name]
	if !ok {
		return false
	}

	overlayPkgSuffixes := []string{"/feedback", "/navigation", "/action"}
	isOverlayPkg := false
	for _, suffix := range overlayPkgSuffixes {
		if strings.HasSuffix(imp, suffix) || imp == suffix[1:] {
			isOverlayPkg = true
			break
		}
	}
	if !isOverlayPkg {
		return false
	}

	overlayTypes := map[string]bool{
		"NewDialog":         true,
		"NewNavDrawer":      true,
		"NewCommandPalette": true,
		"NewPopupPalette":   true,
		"NewNotification":   true,
		"NewTooltip":        true,
	}

	return overlayTypes[sel.Sel.Name]
}

// isAddChildCall returns true when call is an AddChild or AddChildRuntime
// selector expression.
func (r *OverlayMissingMount) isAddChildCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	methodName := sel.Sel.Name
	return methodName == StrAddChild || methodName == StrAddChildRuntime
}

func init() {
	DefaultRegistry.Register(&OverlayMissingMount{})
}
