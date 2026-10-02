package rules

import (
	"go/ast"
	"strings"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
)

// SiblingOverlay flags overlay types mounted as plain children via AddChild
// without a layer attachment (facet.AttachLayer / ZOrder).
type SiblingOverlay struct{}

func (r *SiblingOverlay) ID() string                     { return "LL021" }
func (r *SiblingOverlay) DefaultSeverity() diag.Severity { return diag.SeverityError }
func (r *SiblingOverlay) Description() string {
	return "overlay mounted as a plain child without layer/ZOrder; use a layer attachment instead"
}

func (r *SiblingOverlay) Check(ctx *Context) []*diag.Diagnostic {
	var diags []*diag.Diagnostic

	for _, f := range ctx.Files {
		if isLayoutOrMarksPackage(f) || isRuntimePackage(f) || isGraphPackage(f) {
			continue
		}

		if !isOverlayImport(f) {
			continue
		}

		// Find AddChild calls.
		var addChildSites []ast.Node
		ast.Inspect(f.AST, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if sel.Sel.Name == StrAddChild || sel.Sel.Name == StrAddChildRuntime {
				addChildSites = append(addChildSites, call)
			}
			return true
		})

		if len(addChildSites) == 0 {
			continue
		}

		// Check if the file uses AttachLayer anywhere.
		hasAttachLayer := false
		ast.Inspect(f.AST, func(n ast.Node) bool {
			if hasAttachLayer {
				return false
			}
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if sel.Sel.Name == StrAttachLayer {
				hasAttachLayer = true
				return false
			}
			return true
		})

		// Check each AddChild call — if the argument references something
		// constructed from an overlay package constructor, flag it.
		for _, node := range addChildSites {
			call := node.(*ast.CallExpr)
			for _, arg := range call.Args {
				if isLikelyOverlay(arg, f.Imports) {
					// Skip if AttachLayer is used (the overlay has layer support).
					if hasAttachLayer {
						continue
					}
					diags = append(diags, &diag.Diagnostic{
						RuleID:   r.ID(),
						Severity: r.DefaultSeverity(),
						Pos:      f.Fset.Position(call.Pos()),
						Message:  "overlay mounted as a plain child without a layer attachment; use facet.AttachLayer with a ZBand instead",
						Teach: diag.Teaching{
							Did:      "attached an overlay as a sibling instead of a layered child",
							UseThis:  "facet.AttachLayer with a ZBand",
							IndexRef: "facet.AttachLayer (RX-1 Q4 named z-bands)",
						},
					})
				}
			}
		}
	}

	return diags
}

// isLikelyOverlay reports whether expr is an overlay-package constructor
// result (optionally unwrapped through .Base() / .LayoutRole()). Narrowed
// (RX-2 Q11): hosting an ALREADY-CONSTRUCTED mark via a field reference is a
// structural choice (the demo's in-flow idiom), not a violation — the rule
// fires only where an overlay is constructed and mounted as a plain sibling
// in the same expression.
func isLikelyOverlay(expr ast.Expr, imports loader.ImportTable) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	// Unwrap x.Base() / x.LayoutRole() to the underlying constructor.
	if sel.Sel.Name == "Base" || sel.Sel.Name == "LayoutRole" {
		return isLikelyOverlay(sel.X, imports)
	}
	// An overlay-package constructor: feedback.NewDialog(...) / New*(...).
	if !strings.HasPrefix(sel.Sel.Name, "New") {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	for local, path := range imports {
		if local != id.Name {
			continue
		}
		for _, suffix := range overlayPackageSuffixes {
			if strings.HasSuffix(path, suffix) || path == suffix[1:] {
				return true
			}
		}
	}
	return false
}

func init() {
	DefaultRegistry.Register(&SiblingOverlay{})
}
