package rules

import (
	"go/ast"
	"go/token"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/classify"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/walk"
)

// ReinventContainer flags every child-arranging LayoutRole literal — i.e. a
// LayoutRole whose OnArrange or OnMeasure function body arranges multiple
// child facets.  This is the primary gate rule (LL003, default error).
//
// The diagnostic carries the owning file's AddChild call sites as related
// spans so the author can see both the reinvention and the composition
// surface in one view.
type ReinventContainer struct{}

func (r *ReinventContainer) ID() string                     { return "LL003" }
func (r *ReinventContainer) DefaultSeverity() diag.Severity { return diag.SeverityError }
func (r *ReinventContainer) Description() string {
	return "hand-rolled layout container: child-arranging LayoutRole detected; use an existing layout container or mark"
}

func (r *ReinventContainer) Check(ctx *Context) []*diag.Diagnostic {
	var diags []*diag.Diagnostic

	for _, f := range ctx.Files {
		fid := facetIdent(f.Imports)

		// Collect all LayoutRole literals and AddChild call positions.
		type roleInfo struct {
			lit *ast.CompositeLit
			pos token.Position
		}

		var roles []roleInfo
		var addChildPositions []token.Position

		ast.Inspect(f.AST, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.CompositeLit:
				if walk.CompositeLitIs(n, fid, "LayoutRole") {
					roles = append(roles, roleInfo{
						lit: n,
						pos: f.Fset.Position(n.Pos()),
					})
				}
			case *ast.CallExpr:
				if sel, ok := n.Fun.(*ast.SelectorExpr); ok {
					if sel.Sel.Name == "AddChild" {
						addChildPositions = append(addChildPositions, f.Fset.Position(sel.Pos()))
					}
				}
			}
			return true
		})

		for _, role := range roles {
			if !classify.IsChildArranging(role.lit, f.Fset, f.Imports) {
				continue
			}

			diags = append(diags, &diag.Diagnostic{
				RuleID:   r.ID(),
				Severity: r.DefaultSeverity(),
				Pos:      role.pos,
				Message:  "hand-rolled layout container: child-arranging LayoutRole detected; use an existing layout container or mark instead",
				Teach: diag.Teaching{
					Did:      "wrote a LayoutRole that arranges child facets",
					UseThis:  "structure/panel or another built-in layout container",
					IndexRef: "marks/structure.Panel",
				},
				Related: addChildPositions,
			})
		}
	}

	return diags
}

func init() {
	DefaultRegistry.Register(&ReinventContainer{})
}
