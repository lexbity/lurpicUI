package capindex

import (
	"go/ast"
	"strings"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/walk"
)

// computeFingerprint computes the structural fingerprint of a type spec.
func computeFingerprint(ts *ast.TypeSpec, f *loader.ParsedFile) Fingerprint {
	fp := Fingerprint{}

	// Check if the type embeds facet.Facet or marks.Core (which itself
	// embeds facet.Facet).
	if walk.EmbedsFacet(ts, f.Imports) || embedsMarksCore(ts, f) {
		fp.EmbedsFacet = true
	}

	// Check for child slice fields ([]facet.Facet, []*facet.Facet, etc.).
	fp.HasChildSlice = hasChildSliceField(ts, f)

	// Determine roles by inspecting the struct fields for known role types.
	if st, ok := ts.Type.(*ast.StructType); ok && st.Fields != nil {
		fid := facetLocalIdent(f.Imports)
		for _, field := range st.Fields.List {
			if len(field.Names) != 1 {
				continue
			}
			fieldName := field.Names[0].Name
			if strings.Contains(fieldName, "layout") || strings.Contains(fieldName, "Layout") {
				if walk.SelectorIs(field.Type, fid, "LayoutRole") {
					fp.Roles = append(fp.Roles, "layout")
				}
			}
			if strings.Contains(fieldName, "render") || strings.Contains(fieldName, "Render") {
				fp.Roles = append(fp.Roles, "render")
			}
		}
	}

	// Container heuristic: embeds facet.Facet and has layout role.
	fp.IsContainer = fp.EmbedsFacet && hasRole(fp.Roles, "layout")

	return fp
}

// embedsMarksCore reports whether the struct type embeds marks.Core (which
// itself embeds facet.Facet).
func embedsMarksCore(ts *ast.TypeSpec, f *loader.ParsedFile) bool {
	st, ok := ts.Type.(*ast.StructType)
	if !ok || st.Fields == nil {
		return false
	}
	for _, field := range st.Fields.List {
		if len(field.Names) > 0 {
			continue // named field, not an embed
		}
		sel, ok := field.Type.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		id, ok := sel.X.(*ast.Ident)
		if !ok {
			continue
		}
		if sel.Sel.Name == "Core" && id.Name == "marks" {
			return true
		}
	}
	return false
}

// hasChildSliceField checks whether the struct type has a field that holds
// child facets: a slice of facet.Facet, *facet.Facet, facet.FacetImpl, or
// a slice of a named type that itself embeds facet.Facet.
func hasChildSliceField(ts *ast.TypeSpec, f *loader.ParsedFile) bool {
	st, ok := ts.Type.(*ast.StructType)
	if !ok || st.Fields == nil {
		return false
	}
	for _, field := range st.Fields.List {
		arr, ok := field.Type.(*ast.ArrayType)
		if !ok {
			continue
		}
		// Check element type.
		switch elt := arr.Elt.(type) {
		case *ast.SelectorExpr:
			if elt.Sel.Name == "Facet" || elt.Sel.Name == "FacetImpl" {
				return true
			}
		case *ast.StarExpr:
			if sel, ok := elt.X.(*ast.SelectorExpr); ok && sel.Sel.Name == "Facet" {
				return true
			}
		case *ast.Ident:
			if strings.Contains(elt.Name, "Child") || strings.Contains(elt.Name, "child") {
				return true
			}
		}
	}
	return false
}

// hasRole reports whether the role list contains the given role name.
func hasRole(roles []string, target string) bool {
	for _, r := range roles {
		if r == target {
			return true
		}
	}
	return false
}

// facetLocalIdent returns the local identifier used for the facet package.
func facetLocalIdent(imports map[string]string) string {
	for local, path := range imports {
		if strings.HasSuffix(path, "/facet") || path == "facet" {
			return local
		}
	}
	return "facet"
}
