package rules

import (
	"go/ast"
	"go/token"
	"path/filepath"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/classify"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/walk"
)

// ReinventContainer flags every child-arranging LayoutRole literal — i.e. a
// LayoutRole whose OnArrange or OnMeasure function body arranges multiple
// child facets.  This is the primary gate rule (LL003, default error).
//
// The diagnostic carries the owning file's AddChild call sites as related
// spans so the author can see both the reinvention and the composition
// surface in one view.
//
// Since Slice 1, LL003 also detects field-assigned OnArrange/OnMeasure
// callbacks (e.g. r.layout.OnArrange = func(...){...}) and, via the
// package-local resolver, traces into same-package helper functions
// such as arrangeChildAtCtx before deciding whether children are arranged.
type ReinventContainer struct{}

func (r *ReinventContainer) ID() string                     { return "LL003" }
func (r *ReinventContainer) DefaultSeverity() diag.Severity { return diag.SeverityError }
func (r *ReinventContainer) Description() string {
	return "hand-rolled layout container: child-arranging LayoutRole detected; use an existing layout container or mark"
}

func (r *ReinventContainer) Check(ctx *Context) []*diag.Diagnostic {
	var diags []*diag.Diagnostic

	resolverCache := make(map[string]*classify.PkgFuncResolver)

	for _, f := range ctx.Files {
		if isLayoutOrMarksPackage(f) || isRuntimePackage(f) {
			continue
		}

		fid := facetIdent(f.Imports)

		pkgDir := filepath.Dir(f.Path)
		resolver := resolverCache[pkgDir]
		if resolver == nil {
			if pkg, ok := ctx.Pkgs[pkgDir]; ok {
				resolver = classify.NewPkgFuncResolver(pkg)
				resolverCache[pkgDir] = resolver
			}
		}

		diags = append(diags, r.checkCompositeLiterals(f, fid, resolver)...)
		diags = append(diags, r.checkFieldAssignments(f, fid, resolver)...)
	}

	return diags
}

// checkCompositeLiterals returns the original LL003 findings: child-arranging
// facet.LayoutRole{OnArrange: func(...){...}} / {OnMeasure: func(...){...}}
// composite literals.
func (r *ReinventContainer) checkCompositeLiterals(f *loader.ParsedFile, fid string, resolver *classify.PkgFuncResolver) []*diag.Diagnostic {
	var diags []*diag.Diagnostic

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
		if !classify.IsChildArranging(role.lit, f.Fset, f.Imports, resolver) {
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

	return diags
}

// checkFieldAssignments flags assignments of the form
//
//	<receiver>.<field>.OnMeasure = func(...) {...}
//	<receiver>.<field>.OnArrange  = func(...) {...}
//
// where <field> is a struct field whose declared type is facet.LayoutRole
// and the assigned function literal body arranges child facets.
func (r *ReinventContainer) checkFieldAssignments(f *loader.ParsedFile, fid string, resolver *classify.PkgFuncResolver) []*diag.Diagnostic {
	var diags []*diag.Diagnostic

	layoutFields := layoutRoleFieldNames(f.AST, fid)
	if len(layoutFields) == 0 {
		return diags
	}

	cbs := map[string]bool{"OnMeasure": true, "OnArrange": true}

	ast.Inspect(f.AST, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		if len(assign.Lhs) != len(assign.Rhs) {
			return true
		}
		for i := range assign.Lhs {
			lhs := assign.Lhs[i]
			rhs := assign.Rhs[i]

			// LHS must be <recv>.<field>.<callback>
			sel, ok := lhs.(*ast.SelectorExpr)
			if !ok {
				continue
			}
			if !cbs[sel.Sel.Name] {
				continue
			}
			inner, ok := sel.X.(*ast.SelectorExpr)
			if !ok {
				continue
			}
			if !layoutFields[inner.Sel.Name] {
				continue
			}

			// RHS must be a function literal.
			fn, ok := rhs.(*ast.FuncLit)
			if !ok {
				continue
			}
			if fn.Body == nil {
				continue
			}

			if !classify.BodyArrangesChildren(fn.Body, f.Imports, resolver, 0) {
				continue
			}

			// Collect AddChild positions for related spans.
			var addChildPositions []token.Position
			ast.Inspect(f.AST, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok {
					if sel2, ok2 := call.Fun.(*ast.SelectorExpr); ok2 && sel2.Sel.Name == "AddChild" {
						addChildPositions = append(addChildPositions, f.Fset.Position(sel2.Pos()))
					}
				}
				return true
			})

			diags = append(diags, &diag.Diagnostic{
				RuleID:   r.ID(),
				Severity: r.DefaultSeverity(),
				Pos:      f.Fset.Position(lhs.Pos()),
				Message:  "hand-rolled layout container: field-assigned OnArrange arranges child facets; use an existing layout container or mark instead",
				Teach: diag.Teaching{
					Did:      "assigned a LayoutRole.OnArrange that arranges child facets",
					UseThis:  "structure/panel or a built-in layout container",
					IndexRef: "marks/structure.Panel",
				},
				Related: addChildPositions,
			})
		}
		return true
	})

	return diags
}

func init() {
	DefaultRegistry.Register(&ReinventContainer{})
}
