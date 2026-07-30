package rules

import (
	"go/ast"
	"strings"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
)

// ValueStoreManufacture flags a constructor that accepts a caller-supplied
// *store.ValueStore parameter and also manufactures a fresh store via
// store.NewValueStore. A constructor that takes a caller store MUST NOT
// also manufacture one — doing so orphans the caller's store.
type ValueStoreManufacture struct{}

func (r *ValueStoreManufacture) ID() string                     { return "LL024" }
func (r *ValueStoreManufacture) DefaultSeverity() diag.Severity { return diag.SeverityError }
func (r *ValueStoreManufacture) Description() string {
	return "constructor accepts a caller-supplied store and also manufactures one internally; use the caller's store instead"
}

func (r *ValueStoreManufacture) Explain() string {
	return `LL024: constructor accepts a caller-supplied *store.ValueStore and also
manufactures one via store.NewValueStore.

When a constructor has a *store.ValueStore parameter and creates a new store
via store.NewValueStore for a struct field, the caller's store is orphaned.
The constructor must use the caller-supplied store directly — mutate it with
.Set(), don't replace it.

If the mark needs its own internal store (not caller-supplied), the
constructor should not accept a store parameter at all (like
TreeNavigator/Table/List, which own their Data store).

Example — bad:
    func NewWidget(v *store.ValueStore[State]) *Widget {
        return &Widget{Value: store.NewValueStore(default)}  // orphaning v
    }

Example — good (use caller's store):
    func NewWidget(v *store.ValueStore[State]) *Widget {
        w := &Widget{Value: v}
        w.Value.Set(default)
        return w
    }`
}

// constructorHasStoreParam returns true if fn has any parameter whose type is
// *store.ValueStore[...].
func constructorHasStoreParam(fn *ast.FuncDecl, imports loader.ImportTable) bool {
	return len(storeValueStoreParamFields(fn, imports)) > 0
}

// constructorManufacturesStore returns true if fn's body contains a
// store.NewValueStore(...) call assigned to a struct field (via composite
// literal or assignment).
func constructorManufacturesStore(fn *ast.FuncDecl, imports loader.ImportTable) bool {
	found := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if found {
			return false
		}
		switch e := n.(type) {
		case *ast.CompositeLit:
			for _, elt := range e.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				if isStoreNewValueStoreCall(kv.Value, imports) {
					found = true
					return false
				}
			}
		case *ast.AssignStmt:
			if len(e.Lhs) != 1 || len(e.Rhs) != 1 {
				return true
			}
			if _, ok := e.Lhs[0].(*ast.SelectorExpr); !ok {
				return true
			}
			if isStoreNewValueStoreCall(e.Rhs[0], imports) {
				found = true
				return false
			}
		case *ast.ReturnStmt:
			// &T{Field: store.NewValueStore(...)} in return expressions.
			for _, ret := range e.Results {
				cl, ok := ret.(*ast.CompositeLit)
				if !ok {
					continue
				}
				for _, elt := range cl.Elts {
					kv, ok := elt.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					if isStoreNewValueStoreCall(kv.Value, imports) {
						found = true
						return false
					}
				}
			}
		}
		return true
	})
	return found
}

func (r *ValueStoreManufacture) Check(ctx *Context) []*diag.Diagnostic {
	var diags []*diag.Diagnostic

	for _, pkg := range ctx.Pkgs {
		for _, pf := range pkg.Files {
			// Skip framework packages.
			if isLayoutOrMarksPackage(pf) || isRuntimePackage(pf) || isGraphPackage(pf) {
				continue
			}

			ast.Inspect(pf.AST, func(n ast.Node) bool {
				fn, ok := n.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					return true
				}
				// Only consider constructors.
				if !strings.HasPrefix(fn.Name.Name, "New") {
					return true
				}

				// Condition (a): constructor must have a store-typed param.
				if !constructorHasStoreParam(fn, pf.Imports) {
					return true
				}

				// Condition (b): constructor must not manufacture a store.
				if constructorManufacturesStore(fn, pf.Imports) {
					diags = append(diags, &diag.Diagnostic{
						RuleID:   r.ID(),
						Severity: r.DefaultSeverity(),
						Pos:      pf.Fset.Position(fn.Pos()),
						Message:  "constructor accepts a caller-supplied store and also manufactures one internally; use the caller's store instead",
						Teach: diag.Teaching{
							Did:      "manufactured a store.NewValueStore in a constructor that also accepts a store parameter",
							UseThis:  "use the caller-supplied store directly (mutate with .Set(), don't replace)",
							IndexRef: "store.ValueStore (caller-supplied)",
						},
					})
				}

				return true
			})
		}
	}

	return diags
}

// unionImports aggregates the import tables from all files in a package.
func unionImports(pkg *loader.Package) loader.ImportTable {
	uni := make(loader.ImportTable)
	for _, pf := range pkg.Files {
		for k, v := range pf.Imports {
			uni[k] = v
		}
	}
	return uni
}

func init() {
	DefaultRegistry.Register(&ValueStoreManufacture{})
}
