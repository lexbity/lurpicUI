package rules

import (
	"go/ast"
	"go/token"
	"path/filepath"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/walk"
)

// NoLayoutRole detects types that embed facet.Facet and add children (via
// AddChild) but register no LayoutRole — making the children invisible to the
// measure/arrange pass.  This is the "blank-canvas bug".
//
// Default severity: error (silently broken layout).
type NoLayoutRole struct{}

func (r *NoLayoutRole) ID() string                     { return "LL019" }
func (r *NoLayoutRole) DefaultSeverity() diag.Severity { return diag.SeverityError }
func (r *NoLayoutRole) Description() string {
	return "facet embeds facet.Facet and adds children but registers no LayoutRole; children will never be arranged"
}

func (r *NoLayoutRole) Check(ctx *Context) []*diag.Diagnostic {
	var diags []*diag.Diagnostic

	// Precompute package-level layout fields and facet idents.
	type pkgInfo struct {
		pkg          *loader.Package
		fid          string
		layoutFields map[string]bool
	}
	pkgCache := make(map[string]*pkgInfo)

	for _, f := range ctx.Files {
		if isLayoutOrMarksPackage(f) || isRuntimePackage(f) || isGraphPackage(f) {
			continue
		}

		pkgDir := filepath.Dir(f.Path)
		info := pkgCache[pkgDir]
		if info == nil {
			pkg := ctx.Pkgs[pkgDir]
			if pkg == nil {
				continue
			}
			// Compute layoutFields across all files in the package.
			layoutFields := make(map[string]bool)
			var fid string
			for _, pf := range pkg.Files {
				fid = facetIdent(pf.Imports)
				for k, v := range layoutRoleFieldNames(pf.AST, fid) {
					layoutFields[k] = v
				}
			}
			info = &pkgInfo{pkg: pkg, fid: fid, layoutFields: layoutFields}
			pkgCache[pkgDir] = info
		}

		// Find all type declarations that embed facet.Facet.
		ast.Inspect(f.AST, func(n ast.Node) bool {
			ts, ok := n.(*ast.TypeSpec)
			if !ok {
				return true
			}
			if !walk.EmbedsFacet(ts, f.Imports) {
				return true
			}

			typeName := ts.Name.Name

			// Find constructors across the whole package.
			constructors := pkgConstructors(info.pkg, typeName)
			if len(constructors) == 0 {
				return true
			}

			// Collect signals across all constructors.
			hasAddChild := false
			hasLayoutRoleAddRole := false
			hasRegisterRoles := false
			var addChildSites []token.Position

			for _, fn := range constructors {
				sig := constructorRoleSignals(fn, info.layoutFields)
				if sig.HasAddChild {
					hasAddChild = true
				}
				if sig.HasLayoutRoleAddRole {
					hasLayoutRoleAddRole = true
				}
				if sig.HasRegisterRoles {
					hasRegisterRoles = true
				}
				// Collect AddChild positions.
				if fn.Body != nil {
					ast.Inspect(fn.Body, func(n ast.Node) bool {
						call, ok := n.(*ast.CallExpr)
						if !ok {
							return true
						}
						sel, ok := call.Fun.(*ast.SelectorExpr)
						if !ok {
							return true
						}
						if sel.Sel.Name == "AddChild" && isOnFacetReceiver(sel.X) {
							addChildSites = append(addChildSites, f.Fset.Position(sel.Pos()))
						}
						return true
					})
				}
			}

			// Finding: has AddChild but no AddRole or RegisterRoles.
			if hasAddChild && !hasLayoutRoleAddRole && !hasRegisterRoles {
				diags = append(diags, &diag.Diagnostic{
					RuleID:   r.ID(),
					Severity: r.DefaultSeverity(),
					Pos:      f.Fset.Position(ts.Pos()),
					Message:  "facet embeds facet.Facet and adds children but registers no LayoutRole; children will never be arranged",
					Teach: diag.Teaching{
						Did:      "built a parent facet that participates in no layout",
						UseThis:  "register a LayoutRole via AddRole, or delegate parenting to a layout container like layout.NewColumnLayout",
						IndexRef: "layout.NewColumnLayout",
					},
					Related: addChildSites,
				})
			}

			return true
		})
	}

	return diags
}

func init() {
	DefaultRegistry.Register(&NoLayoutRole{})
}
