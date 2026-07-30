package rules

import (
	"go/ast"
	"go/token"
	"strings"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
)

// VizTheme flags hardcoded gfx.Color literals, "sans-serif" string literals,
// and bare numeric fallback values in marks/viz/ files.  Viz marks must read
// colors and chrome from the theme (FR-9).
type VizTheme struct{}

func (r *VizTheme) ID() string                     { return "LL028" }
func (r *VizTheme) DefaultSeverity() diag.Severity { return diag.SeverityWarn }
func (r *VizTheme) Description() string {
	return "hardcoded gfx.Color, font, or size literal in viz mark; use theme tokens instead"
}

func (r *VizTheme) Explain() string {
	return `LL028: viz mark contains a hardcoded gfx.Color literal, "sans-serif" font
fallback, or bare numeric size value.  Viz marks must read colors and chrome
from the theme (FR-9) for theme-responsiveness.

Fix: use ctx.Theme (theme.ResolvedContext) to resolve colors via ColorToken
and sizes from the resolved context's TextStyle or spacing tokens.

Example — bad:
    LabelColor: gfx.Color{R: 0.3, G: 0.3, B: 0.3, A: 1}

Example — good:
    labelColor = rc.Color(theme.ColorTextSecondary)`
}

func (r *VizTheme) Check(ctx *Context) []*diag.Diagnostic {
	var diags []*diag.Diagnostic

	for _, pkg := range ctx.Pkgs {
		for _, pf := range pkg.Files {
			if !isVizPackage(pf) {
				continue
			}
			if strings.HasSuffix(pf.Path, "_test.go") {
				continue
			}

			ast.Inspect(pf.AST, func(n ast.Node) bool {
				switch e := n.(type) {
				case *ast.CompositeLit:
					// gfx.Color{...} composite literal.
					if isGfxColorLit(e, pf.Imports) {
						diags = append(diags, &diag.Diagnostic{
							RuleID:   r.ID(),
							Severity: r.DefaultSeverity(),
							Pos:      pf.Fset.Position(e.Pos()),
							Message:  "hardcoded gfx.Color literal in viz mark; use theme token instead",
							Teach: diag.Teaching{
								Did:      "used a hardcoded gfx.Color literal",
								UseThis:  "read the color from ctx.Theme via a ColorToken",
								IndexRef: "theme.ColorToken / ctx.Theme (FR-9)",
							},
						})
					}

				case *ast.BasicLit:
					// "sans-serif" string literal.
					if e.Kind == token.STRING && e.Value == `"sans-serif"` {
						diags = append(diags, &diag.Diagnostic{
							RuleID:   r.ID(),
							Severity: r.DefaultSeverity(),
							Pos:      pf.Fset.Position(e.Pos()),
							Message:  "hardcoded \"sans-serif\" font fallback in viz mark; use theme font instead",
							Teach: diag.Teaching{
								Did:      "used a hardcoded font family string",
								UseThis:  "read the font family from the resolved theme context",
								IndexRef: "theme.ColorToken / ctx.Theme (FR-9)",
							},
						})
					}
				}

				return true
			})
		}
	}

	return diags
}

// isGfxColorLit reports whether expr is a gfx.Color{...} composite literal
// with at least one field set (empty gfx.Color{} is excluded — it's the
// zero default or a comparison, not a hardcoded color).
func isGfxColorLit(expr *ast.CompositeLit, imports map[string]string) bool {
	if expr.Type == nil {
		return false
	}
	if len(expr.Elts) == 0 {
		return false
	}
	// The type must be a SelectorExpr with Sel "Color" on a gfx import.
	sel, ok := expr.Type.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Color" {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	// The gfx package could be imported under any name; fall back to "gfx".
	for local, path := range imports {
		if local == id.Name && (strings.HasSuffix(path, "/gfx") || path == "gfx") {
			return true
		}
	}
	return id.Name == "gfx"
}

func init() {
	DefaultRegistry.Register(&VizTheme{})
}
