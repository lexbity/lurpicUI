package capindex

import (
	"go/ast"
	"strings"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/walk"
)

// maxEmbedDepth bounds transitive embedded-type traversal (cycle safety).
const maxEmbedDepth = 8

// computeFingerprint computes the structural fingerprint of a type spec.
//
// The walk includes transitively embedded types via the resolver, so role
// fields promoted through embedding (e.g. marks.Core's Layout/Render fields)
// are counted — without this, nearly every production mark would appear to
// have no roles and no container fingerprint.
//
// IsContainer additionally requires a child-hosting signal (a Children()
// method or a child-slice field) so leaf marks like primitive.Text — which
// embed marks.Core and inherit its roles but host no children — stay leaves.
func computeFingerprint(ts *ast.TypeSpec, f *loader.ParsedFile, pkg *loader.Package, r *typeResolver) Fingerprint {
	fp := Fingerprint{}
	embedsFacet, roles := collectEmbedded(ts, f, r, 0)
	fp.EmbedsFacet = embedsFacet
	fp.Roles = roles

	// Check for child slice fields ([]facet.Facet, []*facet.Facet, etc.).
	fp.HasChildSlice = hasChildSliceField(ts, f)

	// Container heuristic: embeds facet.Facet, has a layout role, and hosts
	// children (a Children() method or a child-slice field).
	fp.IsContainer = fp.EmbedsFacet && hasRole(fp.Roles, "layout") &&
		(fp.HasChildSlice || hasChildrenMethod(pkg, ts.Name.Name))

	return fp
}

// hasChildrenMethod reports whether pkg declares a Children method on typeName.
func hasChildrenMethod(pkg *loader.Package, typeName string) bool {
	if pkg == nil {
		return false
	}
	for _, pf := range pkg.Files {
		for _, decl := range pf.AST.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name == nil || fn.Name.Name != "Children" || fn.Recv == nil {
				continue
			}
			if recvTypeName(fn.Recv) == typeName {
				return true
			}
		}
	}
	return false
}

// recvTypeName extracts the base type name from a method receiver (pointer or
// value, plain or generic).
func recvTypeName(recv *ast.FieldList) string {
	if recv == nil || len(recv.List) == 0 {
		return ""
	}
	typ := recv.List[0].Type
	if star, ok := typ.(*ast.StarExpr); ok {
		typ = star.X
	}
	if idx, ok := typ.(*ast.IndexExpr); ok {
		typ = idx.X
	}
	if idx, ok := typ.(*ast.IndexListExpr); ok {
		typ = idx.X
	}
	id, ok := typ.(*ast.Ident)
	if !ok {
		return ""
	}
	return id.Name
}

// collectEmbedded walks the type spec and every transitively embedded type,
// returning whether facet.Facet is (transitively) embedded and the merged
// role list from all levels.  This sees promoted role fields: a mark embedding
// marks.Core inherits Core's Layout/Render role fields even though they are not
// declared directly on the mark.
func collectEmbedded(ts *ast.TypeSpec, f *loader.ParsedFile, r *typeResolver, depth int) (embedsFacet bool, roles []string) {
	if ts == nil || f == nil || depth > maxEmbedDepth {
		return false, nil
	}
	st, ok := ts.Type.(*ast.StructType)
	if !ok || st.Fields == nil {
		return false, nil
	}
	fid := facetLocalIdent(f.Imports)
	for _, field := range st.Fields.List {
		if len(field.Names) > 0 {
			// Named field: a role field declared on this type.
			if role := roleForField(field, fid); role != "" {
				roles = appendUnique(roles, role)
			}
			continue
		}
		// Anonymous embedded field.
		if isFacetEmbed(field.Type, f.Imports) {
			embedsFacet = true
			continue
		}
		embedded := r.lookup(field.Type, f)
		if embedded == nil {
			continue
		}
		eFacet, eRoles := collectEmbedded(embedded.spec, embedded.file, r, depth+1)
		embedsFacet = embedsFacet || eFacet
		roles = appendUnique(roles, eRoles...)
	}
	return embedsFacet, roles
}

// roleForField returns the role ("layout", "render") contributed by a named
// struct field, or "" for non-role fields.
func roleForField(field *ast.Field, fid string) string {
	if len(field.Names) == 0 {
		return ""
	}
	fieldName := field.Names[0].Name
	if strings.Contains(fieldName, "layout") || strings.Contains(fieldName, "Layout") {
		if walk.SelectorIs(field.Type, fid, "LayoutRole") {
			return "layout"
		}
	}
	if strings.Contains(fieldName, "render") || strings.Contains(fieldName, "Render") {
		return "render"
	}
	return ""
}

// appendUnique appends role to roles unless already present.
func appendUnique(roles []string, extra ...string) []string {
	for _, e := range extra {
		found := false
		for _, r := range roles {
			if r == e {
				found = true
				break
			}
		}
		if !found {
			roles = append(roles, e)
		}
	}
	return roles
}

// isFacetEmbed reports whether the anonymous embedded field type resolves to
// facet.Facet through the file's import table.
func isFacetEmbed(expr ast.Expr, imports loader.ImportTable) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	if sel.Sel.Name != "Facet" {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return isFacetImport(id.Name, imports)
}

// isFacetImport reports whether name is the local identifier used for the facet
// package in the import table, falling back to accepting "facet" when the table
// does not contain an explicit entry (same-package use).
func isFacetImport(name string, imports loader.ImportTable) bool {
	for local, path := range imports {
		if local == name && (strings.HasSuffix(path, "/facet") || path == "facet") {
			return true
		}
	}
	return name == "facet"
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
