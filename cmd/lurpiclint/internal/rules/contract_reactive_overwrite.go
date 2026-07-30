package rules

import (
	"go/ast"
	"go/token"
	"strings"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
)

// ReactiveBindingOverwrite flags a field that was assigned a reactive binding
// (marks.FromStore / marks.FromDerived) in the package and later overwritten
// by a marks.Const inside a handler function, which severs the source of truth.
// Extended (Slice 2) to also flag reassignment of a caller-supplied
// *store.ValueStore field outside the constructor.
type ReactiveBindingOverwrite struct{}

func (r *ReactiveBindingOverwrite) ID() string                     { return "LL023" }
func (r *ReactiveBindingOverwrite) DefaultSeverity() diag.Severity { return diag.SeverityError }
func (r *ReactiveBindingOverwrite) Description() string {
	return "reactive binding overwritten by marks.Const or caller-supplied store reassigned; mutate the store instead"
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
		// Phase 1: existing marks.Const / marks.FromStore / marks.FromDerived detection.
		sites := make(map[string][]site)
		imports := unionImports(pkg)

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

		// Process phase-1 diagnostics (existing marks.Const severance logic).
		diags = append(diags, r.emitConstSeveranceDiags(sites)...)

		// Phase 2: caller-supplied store field reassignment detection.
		callerFields := callerSuppliedStoreFields(pkg, imports)

		for _, pf := range pkg.Files {
			if isLayoutOrMarksPackage(pf) || isRuntimePackage(pf) || isGraphPackage(pf) {
				continue
			}

			ast.Inspect(pf.AST, func(n ast.Node) bool {
				assign, ok := n.(*ast.AssignStmt)
				if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
					return true
				}

				sel, ok := assign.Lhs[0].(*ast.SelectorExpr)
				if !ok || !callerFields[sel.Sel.Name] {
					return true
				}

				// Skip assignments in constructors (they are initialization).
				fnName := funcNameForNode(pf.AST, assign)
				if strings.HasPrefix(fnName, "New") {
					return true
				}

				diags = append(diags, &diag.Diagnostic{
					RuleID:   r.ID(),
					Severity: r.DefaultSeverity(),
					Pos:      pf.Fset.Position(assign.Pos()),
					Message:  "caller-supplied *store.ValueStore field reassigned; mutate the store via .Set() instead",
					Teach: diag.Teaching{
						Did:      "reassigned a caller-supplied store field",
						UseThis:  "mutate the underlying store via .Set() (the source of truth)",
						IndexRef: "store.ValueStore.Set",
					},
				})

				return true
			})
		}
	}

	return diags
}

// emitConstSeveranceDiags runs the existing const-severance logic from the
// collected sites map. Extracted to keep Check readable.
func (r *ReactiveBindingOverwrite) emitConstSeveranceDiags(sites map[string][]site) []*diag.Diagnostic {
	var diags []*diag.Diagnostic

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

	return diags
}

func init() {
	DefaultRegistry.Register(&ReactiveBindingOverwrite{})
}
