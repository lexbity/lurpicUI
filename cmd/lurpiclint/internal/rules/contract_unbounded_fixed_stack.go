package rules

import (
	"go/ast"
	"go/token"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
)

// UnboundedFixedStack flags a ColumnLayout/RowLayout whose children are all
// Fixed (no Flexible) and the layout is not wrapped in a ScrollRegion.
type UnboundedFixedStack struct{}

func (r *UnboundedFixedStack) ID() string                     { return "LL022" }
func (r *UnboundedFixedStack) DefaultSeverity() diag.Severity { return diag.SeverityError }
func (r *UnboundedFixedStack) Description() string {
	return "ColumnLayout/RowLayout stacks only Fixed children with no ScrollRegion; overflow has nowhere to go"
}

func (r *UnboundedFixedStack) Check(ctx *Context) []*diag.Diagnostic {
	var diags []*diag.Diagnostic

	for _, f := range ctx.Files {
		if isLayoutOrMarksPackage(f) || isRuntimePackage(f) || isGraphPackage(f) {
			continue
		}

		type layoutInfo struct {
			pos         token.Pos
			localName   string
			hasFixed    bool
			hasFlexible bool
		}
		var layouts []layoutInfo

		ast.Inspect(f.AST, func(n ast.Node) bool {
			assign, ok := n.(*ast.AssignStmt)
			if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
				return true
			}
			call, ok := assign.Rhs[0].(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if sel.Sel.Name != "NewColumnLayout" && sel.Sel.Name != "NewRowLayout" {
				return true
			}

			var localName string
			if id, ok := assign.Lhs[0].(*ast.Ident); ok {
				localName = id.Name
			} else if se, ok := assign.Lhs[0].(*ast.SelectorExpr); ok {
				localName = se.Sel.Name
			}

			layouts = append(layouts, layoutInfo{
				pos:       call.Pos(),
				localName: localName,
			})
			return true
		})

		if len(layouts) == 0 {
			continue
		}

		// Second pass: classify Add calls outside loops.
		// Use a depth counter that increments on enter, decrements on exit.
		for li := range layouts {
			li := &layouts[li]
			loopDepth := 0

			var inspect func(ast.Node) bool
			inspect = func(n ast.Node) bool {
				switch n.(type) {
				case *ast.ForStmt, *ast.RangeStmt:
					loopDepth++
					defer func() { loopDepth-- }()
				case *ast.FuncLit:
					return false
				}

				call, ok := n.(*ast.CallExpr)
				if ok {
					sel, ok := call.Fun.(*ast.SelectorExpr)
					if ok && sel.Sel.Name == "Add" && loopDepth == 0 && isLayoutReceiver(sel.X, li.localName) {
						if len(call.Args) == 1 {
							arg := call.Args[0]
							if isConstructorCall(arg, "Fixed", "layout") {
								li.hasFixed = true
							}
							if isConstructorCall(arg, "Flexible", "layout") {
								li.hasFlexible = true
							}
						}
					}
				}
				return true
			}
			ast.Inspect(f.AST, inspect)
		}

		for _, li := range layouts {
			if li.hasFixed && !li.hasFlexible {
				wrapped := false
				if li.localName != "" {
					ast.Inspect(f.AST, func(n ast.Node) bool {
						call, ok := n.(*ast.CallExpr)
						if !ok {
							return true
						}
						sel, ok := call.Fun.(*ast.SelectorExpr)
						if !ok || sel.Sel.Name != "SetChildren" {
							return true
						}
						for _, arg := range call.Args {
							if referencesLocal(arg, li.localName) {
								wrapped = true
								return false
							}
						}
						return true
					})
				}
				if wrapped {
					continue
				}

				diags = append(diags, &diag.Diagnostic{
					RuleID:   r.ID(),
					Severity: r.DefaultSeverity(),
					Pos:      f.Fset.Position(li.pos),
					Message:  "ColumnLayout/RowLayout stacks Fixed children with no Flexible/ScrollRegion; overflow has nowhere to go",
					Teach: diag.Teaching{
						Did:      "built an all-Fixed linear stack without an overflow plan",
						UseThis:  "wrap in structure.ScrollRegion, or add a Flexible child",
						IndexRef: "structure.NewScrollRegion",
					},
				})
			}
		}
	}

	return diags
}

func isLayoutReceiver(expr ast.Expr, localName string) bool {
	if id, ok := expr.(*ast.Ident); ok && id.Name == localName {
		return true
	}
	if sel, ok := expr.(*ast.SelectorExpr); ok && sel.Sel.Name == localName {
		return true
	}
	return false
}

func isConstructorCall(expr ast.Expr, name string, pkgHint string) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	return sel.Sel.Name == name
}

func referencesLocal(expr ast.Expr, localName string) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		if found {
			return false
		}
		if id, ok := n.(*ast.Ident); ok && id.Name == localName {
			found = true
			return false
		}
		if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == localName {
			found = true
			return false
		}
		return true
	})
	return found
}

func init() {
	DefaultRegistry.Register(&UnboundedFixedStack{})
}
