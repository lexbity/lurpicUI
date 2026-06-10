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
func looksLikeDomainState(field *ast.Field, pf *loader.ParsedFile) bool {
	switch t := field.Type.(type) {
	case *ast.ArrayType:
		// Slice of non-ident types (likely domain structs).
		if _, ok := t.Elt.(*ast.Ident); !ok {
			return true
		}
	case *ast.SelectorExpr:
		// Type from another package — check the import for store/domain.
		if id, ok := t.X.(*ast.Ident); ok {
			if importPath, exists := pf.Imports[id.Name]; exists {
				if strings.Contains(importPath, "/store") || strings.Contains(importPath, "/domain") {
					return true
				}
			}
		}
	case *ast.StarExpr:
		// Pointer to selector type from store/domain.
		if sel, ok := t.X.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok {
				if importPath, exists := pf.Imports[id.Name]; exists {
					if strings.Contains(importPath, "/store") || strings.Contains(importPath, "/domain") {
						return true
					}
				}
			}
		}
	}
	return false
}

func init() {
	DefaultRegistry.Register(&DomainStateInFacet{})
}
