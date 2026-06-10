package rules

import (
	"go/ast"
	"go/token"
	"strings"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
)

// OverlayMissingContract flags overlay-like facets that lack a proper layer
// registration, hit policy, or dismissal trigger.  V2 Rule 6 requires
// overlays to declare all three.
//
// Detection heuristic: a type whose name or fields suggest an overlay (name
// contains "overlay" or "Overlay", or has an OverlayRole field) should also
// reference a layer above StandardLayer_Base, a hit role, and a dismissal
// trigger.
//
// Default severity: error.
type OverlayMissingContract struct{}

func (r *OverlayMissingContract) ID() string                     { return "LL014" }
func (r *OverlayMissingContract) DefaultSeverity() diag.Severity { return diag.SeverityError }
func (r *OverlayMissingContract) Description() string {
	return "overlay mark missing layer registration, hit policy, or dismissal trigger (V2 Rule 6)"
}

func (r *OverlayMissingContract) Check(ctx *Context) []*diag.Diagnostic {
	var diags []*diag.Diagnostic

	for _, f := range ctx.Files {
		if !fileContainsFacetType(f) {
			continue
		}

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
				if !looksLikeOverlay(ts) {
					continue
				}

				missing := checkOverlayContracts(ts, f)

				if len(missing) > 0 {
					diags = append(diags, &diag.Diagnostic{
						RuleID:   r.ID(),
						Severity: r.DefaultSeverity(),
						Pos:      f.Fset.Position(ts.Pos()),
						Message:  "overlay " + ts.Name.Name + " missing: " + strings.Join(missing, ", ") + " (V2 Rule 6)",
						Teach: diag.Teaching{
							Did:      "defined an overlay without required contracts",
							UseThis:  "add a layer above StandardLayer_Base, a HitRole, and a DismissalTrigger",
							IndexRef: "",
						},
					})
				}
			}
		}
	}

	return diags
}

// looksLikeOverlay reports whether the type name or fields suggest an overlay.
func looksLikeOverlay(ts *ast.TypeSpec) bool {
	name := ts.Name.Name
	if strings.Contains(name, "overlay") || strings.Contains(name, "Overlay") {
		return true
	}
	st, ok := ts.Type.(*ast.StructType)
	if !ok || st.Fields == nil {
		return false
	}
	for _, field := range st.Fields.List {
		if len(field.Names) == 0 {
			continue
		}
		if field.Names[0].Name == "OverlayRole" || strings.Contains(field.Names[0].Name, "Overlay") {
			return true
		}
	}
	return false
}

// checkOverlayContracts checks which overlay contracts are present.
// Returns a list of missing contract descriptions.
func checkOverlayContracts(ts *ast.TypeSpec, pf *loader.ParsedFile) []string {
	var missing []string

	st, ok := ts.Type.(*ast.StructType)
	if !ok || st.Fields == nil {
		return []string{"layer, hit policy, dismissal trigger"}
	}

	hasLayer := false
	hasHit := false
	hasDismissal := false

	for _, field := range st.Fields.List {
		if len(field.Names) == 0 {
			continue
		}
		name := field.Names[0].Name
		typeStr := typeString(field.Type)

		if strings.Contains(typeStr, "OverlayRole") || strings.Contains(typeStr, "Layer") || strings.Contains(name, "Layer") {
			hasLayer = true
		}
		if strings.Contains(typeStr, "HitRole") || strings.Contains(name, "Hit") {
			hasHit = true
		}
		if strings.Contains(typeStr, "Dismissal") || strings.Contains(name, "Dismiss") {
			hasDismissal = true
		}
	}

	if !hasLayer {
		missing = append(missing, "layer registration")
	}
	if !hasHit {
		missing = append(missing, "hit policy")
	}
	if !hasDismissal {
		missing = append(missing, "dismissal trigger")
	}

	return missing
}

// typeString returns a string representation of an expression for heuristic
// matching.
func typeString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return typeString(t.X) + "." + t.Sel.Name
	case *ast.StarExpr:
		return "*" + typeString(t.X)
	default:
		return ""
	}
}

func init() {
	DefaultRegistry.Register(&OverlayMissingContract{})
}
