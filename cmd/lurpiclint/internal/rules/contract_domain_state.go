package rules

import (
	"go/ast"
	"go/token"
	"strings"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
)

// DomainStateInFacet flags facet-embedding types that hold domain-like
// state as fields.  Runtime Principles 1 and 8 require facets to be
// projection-only and stateless with respect to domain data.
//
// Detection is heuristic: a field is considered domain-state when its type
// is a slice of non-primitive types or its import path suggests a store or
// domain package.
//
// Default severity: warn.
type DomainStateInFacet struct{}

func (r *DomainStateInFacet) ID() string                     { return "LL012" }
func (r *DomainStateInFacet) DefaultSeverity() diag.Severity { return diag.SeverityWarn }
func (r *DomainStateInFacet) Description() string {
	return "facet holds domain state in a field; keep facets stateless (Principles 1 and 8)"
}

func (r *DomainStateInFacet) Check(ctx *Context) []*diag.Diagnostic {
	var diags []*diag.Diagnostic

	for _, f := range ctx.Files {
		if !fileContainsFacetType(f) {
			continue
		}

		for _, decl := range f.AST.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok || st.Fields == nil {
					continue
				}

				for _, field := range st.Fields.List {
					if len(field.Names) == 0 {
						continue // embedded field
					}
					if looksLikeDomainState(field, f) {
						diags = append(diags, &diag.Diagnostic{
							RuleID:   r.ID(),
							Severity: r.DefaultSeverity(),
							Pos:      f.Fset.Position(field.Pos()),
							Message:  "field " + field.Names[0].Name + " looks like domain state; facets should be stateless (Principles 1 and 8)",
							Teach: diag.Teaching{
								Did:      "stored domain-like data in a facet field",
								UseThis:  "keep domain state in a store, not in a facet field",
								IndexRef: "",
							},
						})
					}
				}
			}
		}
	}

	return diags
}

// looksLikeDomainState heuristically checks whether a struct field looks
// like domain data rather than projection/rendering state.
//
// Narrowed (RX-2 Q11): facet-composition fields are NOT domain state —
// a facet embedding hosts child facets as real tree members, so slices of
// facet-typed elements ([]facet.FacetImpl) and lifecycle-closure slices
// ([]func()) are composition structure. Fields named cached* are the
// framework's projection-cache idiom, also not domain state.
func looksLikeDomainState(field *ast.Field, pf *loader.ParsedFile) bool {
	if len(field.Names) > 0 && strings.HasPrefix(field.Names[0].Name, "cached") {
		return false
	}
	fromStoreDomain := func(expr ast.Expr) bool {
		if id, ok := expr.(*ast.Ident); ok {
			if importPath, exists := pf.Imports[id.Name]; exists {
				return strings.Contains(importPath, "/store") || strings.Contains(importPath, "/domain")
			}
		}
		return false
	}
	switch t := field.Type.(type) {
	case *ast.ArrayType:
		// Flag only slices whose element type is store/domain state
		// ([]*store.ValueStore[T], []domain.Row). Facet-composition slices
		// ([]facet.FacetImpl) and lifecycle closures ([]func()) are not
		// domain state. Unwrap pointer and index layers so generic store
		// handles ([]*store.ValueStore[T]) resolve to their selector root.
		elt := t.Elt
		for {
			switch e := elt.(type) {
			case *ast.StarExpr:
				elt = e.X
			case *ast.IndexExpr:
				elt = e.X
			default:
				goto checkedElement
			}
		}
	checkedElement:
		if sel, ok := elt.(*ast.SelectorExpr); ok {
			return fromStoreDomain(sel.X)
		}
		return false
	case *ast.SelectorExpr:
		// Type from another package — check the import for store/domain.
		return fromStoreDomain(t.X)
	case *ast.StarExpr:
		// Pointer to selector type from store/domain.
		if sel, ok := t.X.(*ast.SelectorExpr); ok {
			// Unwrap index layers so *store.ValueStore[T] resolves too,
			// but only when the star directly wraps a selector (a bare
			// *store handle field is the framework's store-injection
			// idiom, not domain state — singleton handles stay clean).
			return fromStoreDomain(sel.X)
		}
	}
	return false
}

func init() {
	DefaultRegistry.Register(&DomainStateInFacet{})
}
