package rules

import (
	"go/ast"
	"go/token"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
)

// ReactiveBindingOverwrite flags a field that was assigned a reactive binding
// (marks.FromStore / marks.FromDerived) in the package and later overwritten
// by a marks.Const inside a handler function, which severs the source of truth.
type ReactiveBindingOverwrite struct{}

func (r *ReactiveBindingOverwrite) ID() string                     { return "LL023" }
func (r *ReactiveBindingOverwrite) DefaultSeverity() diag.Severity { return diag.SeverityError }
func (r *ReactiveBindingOverwrite) Description() string {
	return "reactive binding overwritten by marks.Const; mutate the store instead"
}

// funcNameForNode finds the enclosing function name for a node in a file.
func funcNameForNode(file *ast.File, target ast.Node) string {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		if fn.Pos() <= target.Pos() && target.End() <= fn.End() {
			return fn.Name.Name
		}
	}
	return ""
}

// site records one assignment to a binding field.
type site struct {
	pos      token.Position
	isConst  bool
	funcName string // enclosing function name, empty if unknown
}

func (r *ReactiveBindingOverwrite) Check(ctx *Context) []*diag.Diagnostic {
	var diags []*diag.Diagnostic

	for _, pkg := range ctx.Pkgs {
		sites := make(map[string][]site)

		for _, pf := range pkg.Files {
			// Skip framework packages (allowlist gate).
			if isLayoutOrMarksPackage(pf) || isRuntimePackage(pf) || isGraphPackage(pf) {
				continue
			}

			ast.Inspect(pf.AST, func(n ast.Node) bool {
				assign, ok := n.(*ast.AssignStmt)
				if !ok || len(assign.Lhs) != 1 {
					return true
				}
				lhs := assign.Lhs[0]
				rhs := assign.Rhs[0]

				sel, ok := lhs.(*ast.SelectorExpr)
				if !ok {
					return true
				}

				call, ok := rhs.(*ast.CallExpr)
				if !ok {
					return true
				}
				callSel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}

				// RHS must resolve to a marks.Const / marks.FromStore /
				// marks.FromDerived call through the import table.
				if !isMarksConstruct(callSel, pf.Imports) {
					return true
				}

				fnName := funcNameForNode(pf.AST, assign)

				sites[sel.Sel.Name] = append(sites[sel.Sel.Name], site{
					pos:      pf.Fset.Position(assign.Pos()),
					isConst:  callSel.Sel.Name == "Const",
					funcName: fnName,
				})

				return true
			})
		}

		for _, s := range sites {
			type funcSites struct {
				constSites    []site
				reactiveSites []site
			}
			byFunc := make(map[string]*funcSites)
			for _, site := range s {
				fn := site.funcName
				if byFunc[fn] == nil {
					byFunc[fn] = &funcSites{}
				}
				if site.isConst {
					byFunc[fn].constSites = append(byFunc[fn].constSites, site)
				} else {
					byFunc[fn].reactiveSites = append(byFunc[fn].reactiveSites, site)
				}
			}

			hasReactiveAnywhere := false
			for _, fs := range byFunc {
				if len(fs.reactiveSites) > 0 {
					hasReactiveAnywhere = true
				}
			}
			if !hasReactiveAnywhere {
				continue
			}

			var reactiveSites []token.Position
			for _, fs := range byFunc {
				for _, site := range fs.reactiveSites {
					reactiveSites = append(reactiveSites, site.pos)
				}
			}

			for fn, fs := range byFunc {
				for _, site := range fs.constSites {
					if len(byFunc[fn].reactiveSites) > 0 {
						continue
					}
					diags = append(diags, &diag.Diagnostic{
						RuleID:   r.ID(),
						Severity: r.DefaultSeverity(),
						Pos:      site.pos,
						Message:  "reactive binding overwritten by marks.Const, severing the source of truth; mutate the store instead",
						Teach: diag.Teaching{
							Did:      "overwrote a reactive binding with a constant",
							UseThis:  "mutate the underlying store/derived (the source of truth)",
							IndexRef: "store.ValueStore.Set",
						},
						Related: reactiveSites,
					})
				}
			}
		}
	}

	return diags
}

func init() {
	DefaultRegistry.Register(&ReactiveBindingOverwrite{})
}
