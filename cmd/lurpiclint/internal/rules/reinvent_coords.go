package rules

import (
	"go/ast"
	"go/token"
	"strings"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/classify"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/walk"
)

// ReinventCoords flags absolute-coordinate placement (gfx.RectFromXYWH)
// inside LayoutRole's OnArrange or OnMeasure when the role is not
// child-arranging (LL003 would already fire on those).
//
// Detection:
//   - 2+ RectFromXYWH calls in a callback body → report.
//   - 1 RectFromXYWH with non-trivial (computed) arguments → report.
//   - 1 RectFromXYWH inside a for/range loop → report.
//   - 1 RectFromXYWH with only constant arguments, outside a loop → suppress
//     (legitimate leaf mark drawing its own geometry).
type ReinventCoords struct{}

func (r *ReinventCoords) ID() string                     { return "LL002" }
func (r *ReinventCoords) DefaultSeverity() diag.Severity { return diag.SeverityWarn }
func (r *ReinventCoords) Description() string {
	return "absolute-coordinate placement via gfx.RectFromXYWH in an arrange path; prefer relative layout"
}

func (r *ReinventCoords) Check(ctx *Context) []*diag.Diagnostic {
	var diags []*diag.Diagnostic

	for _, f := range ctx.Files {
		fid := facetIdent(f.Imports)
		gid := gfxIdent(f.Imports)

		type roleInfo struct {
			lit *ast.CompositeLit
			pos token.Position
		}

		var roles []roleInfo

		ast.Inspect(f.AST, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			if walk.CompositeLitIs(lit, fid, "LayoutRole") {
				roles = append(roles, roleInfo{
					lit: lit,
					pos: f.Fset.Position(lit.Pos()),
				})
			}
			return true
		})

		for _, role := range roles {
			// De-dup: if this LayoutRole is child-arranging, LL003
			// already fires; skip LL002 to avoid noise.
			if classify.IsChildArranging(role.lit, f.Fset, f.Imports) {
				continue
			}

			// Inspect OnArrange and OnMeasure bodies.
			for _, key := range []string{"OnArrange", "OnMeasure"} {
				val := walk.KeyValue(role.lit, key)
				if val == nil {
					continue
				}
				body := walk.FuncLitBody(val)
				if body == nil {
					continue
				}

				rectCalls := walk.FindCallExprs(body, func(call *ast.CallExpr) bool {
					return walk.SelectorIs(call.Fun, gid, "RectFromXYWH")
				})

				if len(rectCalls) == 0 {
					continue
				}

				// Check if any call is inside a for/range loop.
				insideLoop := false
				for _, call := range rectCalls {
					if isInsideLoop(body, call) {
						insideLoop = true
						break
					}
				}

				for _, call := range rectCalls {
					shouldReport := false

					if len(rectCalls) >= 2 {
						// Multiple rect constructions.
						shouldReport = true
					} else if insideLoop {
						// Single rect inside a loop.
						shouldReport = true
					} else if hasNonTrivialArgs(call) {
						// Single rect with computed arguments.
						shouldReport = true
					}

					if !shouldReport {
						continue
					}

					diags = append(diags, &diag.Diagnostic{
						RuleID:   r.ID(),
						Severity: r.DefaultSeverity(),
						Pos:      f.Fset.Position(call.Pos()),
						Message:  "absolute coordinate placement via gfx.RectFromXYWH; prefer relative layout or an existing container",
						Teach: diag.Teaching{
							Did:      "placed a child at absolute coordinates using gfx.RectFromXYWH",
							UseThis:  "a built-in layout container (structure/panel, layout/linear, etc.)",
							IndexRef: "marks/structure.Panel",
						},
					})

					// Report at most one diagnostic per callback body.
					break
				}
			}
		}
	}

	return diags
}

// hasNonTrivialArgs returns true if any argument to the call is not a basic
// literal (i.e. it's a variable reference, expression, or function call).
func hasNonTrivialArgs(call *ast.CallExpr) bool {
	for _, arg := range call.Args {
		if _, ok := arg.(*ast.BasicLit); !ok {
			return true
		}
	}
	return false
}

// isInsideLoop reports whether node is contained within a for or range
// statement in body.
func isInsideLoop(body ast.Node, target ast.Node) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		switch n := n.(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			if nodeContains(n, target) {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

// nodeContains reports whether the root node contains the target node by
// comparing positions.
func nodeContains(root, target ast.Node) bool {
	if root == nil || target == nil {
		return false
	}
	return root.Pos() <= target.Pos() && target.End() <= root.End()
}

// gfxIdent returns the local identifier for the gfx package in the import
// table, falling back to "gfx".
func gfxIdent(imports map[string]string) string {
	for local, path := range imports {
		if strings.HasSuffix(path, "/gfx") || path == "gfx" {
			return local
		}
	}
	return "gfx"
}

func init() {
	DefaultRegistry.Register(&ReinventCoords{})
}
