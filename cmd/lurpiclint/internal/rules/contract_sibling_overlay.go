package rules

import (
	"go/ast"
	"strings"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
)

// SiblingOverlay flags overlay types mounted as plain children via AddChild
// without a layer attachment (facet.AttachLayer / ZPriority).
type SiblingOverlay struct{}

func (r *SiblingOverlay) ID() string                     { return "LL021" }
func (r *SiblingOverlay) DefaultSeverity() diag.Severity { return diag.SeverityError }
func (r *SiblingOverlay) Description() string {
	return "overlay mounted as a plain child without layer/ZPriority; use a layer attachment instead"
}

// overlayPackageSuffixes are import path suffixes that contain overlay marks.
var overlayPackageSuffixes = []string{
	"/marks/feedback",
	"/marks/action",
	"/marks/navigation",
}

func (r *SiblingOverlay) Check(ctx *Context) []*diag.Diagnostic {
	var diags []*diag.Diagnostic

	for _, f := range ctx.Files {
		if isLayoutOrMarksPackage(f) || isRuntimePackage(f) || isGraphPackage(f) {
			continue
		}

		// Check if any overlay package is imported.
		hasOverlayImport := false
		for _, path := range f.Imports {
			for _, suffix := range overlayPackageSuffixes {
				if strings.HasSuffix(path, suffix) || path == suffix[1:] {
					hasOverlayImport = true
					break
				}
			}
			if hasOverlayImport {
				break
			}
		}
		if !hasOverlayImport {
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
			if sel.Sel.Name == "AddChild" || sel.Sel.Name == "AddChildRuntime" {
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
			if sel.Sel.Name == "AttachLayer" {
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
						Message:  "overlay mounted as a plain child without layer/ZPriority; use facet.AttachLayer with a ZPriority instead",
						Teach: diag.Teaching{
							Did:      "attached an overlay as a sibling instead of a layered child",
							UseThis:  "facet.AttachLayer with a ZPriority",
							IndexRef: "layout.NewOverlayLayer",
						},
					})
				}
			}
		}
	}

	return diags
}

// isLikelyOverlay reports whether expr references a value likely constructed
// from an overlay package.  It checks the import table for overlay packages
// and checks if the expression matches known overlay patterns.
func isLikelyOverlay(expr ast.Expr, imports loader.ImportTable) bool {
	// Unwrap method calls like x.Base(), x.LayoutRole().
	if call, ok := expr.(*ast.CallExpr); ok {
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			// Recursion into the receiver: r.dialog.Base() -> r.dialog
			if sel.Sel.Name == "Base" || sel.Sel.Name == "LayoutRole" {
				return isLikelyOverlay(sel.X, imports)
			}
		}
	}

	// Check selector references: r.dialog, r.exportToast, etc.
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		// If the field name looks like it could be an overlay type.
		if overlayTypeNames[sel.Sel.Name] {
			return true
		}
		// Check if the file imports from the overlay package that this
		// selector chain originates from.
		if id, ok := sel.X.(*ast.Ident); ok {
			for local, path := range imports {
				if local == id.Name {
					for _, suffix := range overlayPackageSuffixes {
						if strings.HasSuffix(path, suffix) || path == suffix[1:] {
							return true
						}
					}
				}
			}
		}
		return isLikelyOverlay(sel.X, imports)
	}

	// Direct ident (less likely for overlays but check).
	if _, ok := expr.(*ast.Ident); ok {
		return true // conservative: any ident could be an overlay
	}

	return false
}

// overlayTypeNames are well-known overlay type names that can be used as field names.
var overlayTypeNames = map[string]bool{
	"dialog":         true,
	"notification":   true,
	"tooltip":        true,
	"commandPalette": true,
	"popupPalette":   true,
	"navDrawer":      true,
	"Dialog":         true,
	"Notification":   true,
	"Tooltip":        true,
	"CommandPalette": true,
	"PopupPalette":   true,
	"NavDrawer":      true,
}

func init() {
	DefaultRegistry.Register(&SiblingOverlay{})
}
