package rules

import (
	"strings"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
)

// FacetImportsRender flags any file under the facet/ or projection/ package
// tree that imports the framework's render package.  This enforces Runtime
// Principle 3: the projection and facet layers must not depend on the render
// backend.
//
// Default severity: error.
type FacetImportsRender struct{}

func (r *FacetImportsRender) ID() string                     { return "LL010" }
func (r *FacetImportsRender) DefaultSeverity() diag.Severity { return diag.SeverityError }
func (r *FacetImportsRender) Description() string {
	return "facet or projection package must not import render"
}

const moduleRenderPrefix = "codeburg.org/lexbit/lurpicui/render"

func (r *FacetImportsRender) Check(ctx *Context) []*diag.Diagnostic {
	var diags []*diag.Diagnostic

	for _, f := range ctx.Files {
		if f.Pkg != "facet" && f.Pkg != "projection" {
			continue
		}

		for _, spec := range f.AST.Imports {
			if spec.Path == nil {
				continue
			}
			importPath := strings.Trim(spec.Path.Value, "\"")
			if !strings.HasPrefix(importPath, moduleRenderPrefix) {
				continue
			}

			diags = append(diags, &diag.Diagnostic{
				RuleID:   r.ID(),
				Severity: r.DefaultSeverity(),
				Pos:      f.Fset.Position(spec.Pos()),
				Message:  "facet or projection package imports render backend; the projection layer must not depend on the render backend (Principle 3)",
				Teach: diag.Teaching{
					Did:      "imported the render package from a facet or projection package",
					UseThis:  "keep render imports confined to the render backend and its adapters",
					IndexRef: "",
				},
			})
		}
	}

	return diags
}

func init() {
	DefaultRegistry.Register(&FacetImportsRender{})
}
