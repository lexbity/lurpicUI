package rules

import (
	"go/ast"
	"strings"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/walk"
)

// ReinventLayoutRole flags every non-empty facet.LayoutRole composite literal
// whose OnMeasure or OnArrange field is explicitly set.  Even a legitimate
// custom mark should prefer composition over raw role population.
//
// Default severity: warn (leaf marks are not penalised; the hard gate is
// LL003 which fires only when the role arranges children).
type ReinventLayoutRole struct{}

func (r *ReinventLayoutRole) ID() string                     { return "LL001" }
func (r *ReinventLayoutRole) DefaultSeverity() diag.Severity { return diag.SeverityWarn }
func (r *ReinventLayoutRole) Description() string {
	return "raw LayoutRole literal with OnMeasure or OnArrange set (prefer composition)"
}

func (r *ReinventLayoutRole) Check(ctx *Context) []*diag.Diagnostic {
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

			hasOnMeasure := walk.KeyValue(lit, "OnMeasure") != nil
			hasOnArrange := walk.KeyValue(lit, "OnArrange") != nil
			if !hasOnMeasure && !hasOnArrange {
				return true
			}

			diags = append(diags, &diag.Diagnostic{
				RuleID:   r.ID(),
				Severity: r.DefaultSeverity(),
				Pos:      f.Fset.Position(lit.Pos()),
				Message:  "raw LayoutRole literal with OnMeasure or OnArrange set; prefer using an existing layout container or mark",
				Teach: diag.Teaching{
					Did:      "populated a LayoutRole struct directly",
					UseThis:  "an existing layout container or mark such as structure/panel",
					IndexRef: "marks/structure.Panel",
				},
			})
			return true
		})
	}

	return diags
}

// facetIdent returns the local identifier used for the "facet" package in
// the import table, falling back to "facet" when no import entry matches.
func facetIdent(imports map[string]string) string {
	for local, path := range imports {
		if strings.HasSuffix(path, "/facet") || path == "facet" {
			return local
		}
	}
	return "facet"
}

func init() {
	DefaultRegistry.Register(&ReinventLayoutRole{})
}
