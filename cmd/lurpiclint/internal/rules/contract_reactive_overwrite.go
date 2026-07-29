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

// bindingFieldNames are field names known to carry Binding[T] values on mark types.
var bindingFieldNames = map[string]bool{
	"Open": true, "Value": true, "Content": true, "Label": true,
	"Title": true, "Body": true, "Actions": true, "Disabled": true,
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
	pos       token.Position
	isConst   bool
	funcName  string // enclosing function name, empty if unknown
}

func (r *ReactiveBindingOverwrite) Check(ctx *Context) []*diag.Diagnostic {
	var diags []*diag.Diagnostic

	for _, pkg := range ctx.Pkgs {
		sites := make(map[string][]site)

		for _, pf := range pkg.Files {
			ast.Inspect(pf.AST, func(n ast.Node) bool {
				assign, ok := n.(*ast.AssignStmt)
				if !ok || len(assign.Lhs) != 1 {
					return true
				}
				lhs := assign.Lhs[0]
				rhs := assign.Rhs[0]

				sel, ok := lhs.(*ast.SelectorExpr)
				if !ok || !bindingFieldNames[sel.Sel.Name] {
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

				fnName := funcNameForNode(pf.AST, assign)

				switch callSel.Sel.Name {
				case "Const":
					sites[sel.Sel.Name] = append(sites[sel.Sel.Name], site{
						pos:      pf.Fset.Position(assign.Pos()),
						isConst:  true,
						funcName: fnName,
					})
				case "FromStore", "FromDerived":
					sites[sel.Sel.Name] = append(sites[sel.Sel.Name], site{
						pos:      pf.Fset.Position(assign.Pos()),
						isConst:  false,
						funcName: fnName,
					})
				}
				return true
			})
		}

		for _, s := range sites {
			// Group by function: track const/reactive per function.
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

			// A const site is an overwrite if:
			// 1. It's in a handler function (on*, handle*, toggle*) AND
			//    a reactive binding exists in any OTHER function.
			// 2. OR it's in a function that has NO reactive site for this field
			//    but a reactive site exists in a different function.
			hasReactiveAnywhere := false
			for _, fs := range byFunc {
				if len(fs.reactiveSites) > 0 {
					hasReactiveAnywhere = true
				}
			}
			if !hasReactiveAnywhere {
				continue
			}

			// Collect reactive sites for Related span.
			var reactiveSites []token.Position
			for _, fs := range byFunc {
				for _, site := range fs.reactiveSites {
					reactiveSites = append(reactiveSites, site.pos)
				}
			}

			// Emit at const sites that are overwrites.
			for fn, fs := range byFunc {
				for _, site := range fs.constSites {
					// Skip if this function ALSO has reactive sites for the same
					// field — it's a constructor initializer, not an overwrite.
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
