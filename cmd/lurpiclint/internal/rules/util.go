package rules

import (
	"go/ast"
	"path/filepath"
	"strings"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
)

// facetIdent returns the local identifier used for the "facet" package in
// the import table, falling back to "facet" when no import entry matches.
func facetIdent(imports map[string]string) string {
	for local, path := range imports {
		if strings.HasSuffix(path, "/facet") || path == "facet" {
			return local
		}
	}
	return "facet"
}

// layoutRoleFieldNames returns the set of struct field names declared with
// type facet.LayoutRole anywhere in the file.  Used by LL001 to recognise
// field-assignment patterns such as  r.layout.OnMeasure = func(...) {...}
// without resorting to full type-checking.
//
// A field qualifies when its declared type is either:
//   - a bare ident named "LayoutRole" (same-package or aliased), or
//   - <facetIdent>.LayoutRole, where the import table maps <facetIdent>
//     to a path ending in "/facet".
func layoutRoleFieldNames(root ast.Node, fid string) map[string]bool {
	out := map[string]bool{}

	ast.Inspect(root, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}
		st, ok := ts.Type.(*ast.StructType)
		if !ok || st.Fields == nil {
			return true
		}
		for _, field := range st.Fields.List {
			if !isLayoutRoleType(field.Type, fid) {
				continue
			}
			for _, name := range field.Names {
				if name != nil {
					out[name.Name] = true
				}
			}
		}
		return true
	})

	return out
}

// isLayoutRoleType reports whether expr is a type expression spelling out
// facet.LayoutRole (or a same-package LayoutRole ident when the file does
// not alias the import).
func isLayoutRoleType(expr ast.Expr, fid string) bool {
	// Bare ident: LayoutRole — same-package use, accepted.
	if id, ok := expr.(*ast.Ident); ok && id.Name == "LayoutRole" {
		return true
	}
	// Selector: <facetIdent>.LayoutRole.
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	if sel.Sel.Name != "LayoutRole" {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return id.Name == fid
}

// isLayoutOrMarksPackage reports whether the file lives inside the layout/
// or marks/ package tree, where LayoutRole callback assignment is
// legitimate.  The decision is based on the file's own directory path only:
// scanning the file's import table would falsely suppress any consumer that
// merely imports layout/ or marks/.
func isLayoutOrMarksPackage(f *loader.ParsedFile) bool {
	cleanPath := filepath.ToSlash(f.Path)

	return strings.Contains(cleanPath, "/layout/") ||
		strings.Contains(cleanPath, "/marks/") ||
		strings.HasPrefix(cleanPath, "layout/") ||
		strings.HasPrefix(cleanPath, "marks/")
}

// isDemoPackage reports whether the file belongs to a demo-style package
// (name-based or import-signature match).  Demo packages are subject to
// additional concurrency restrictions (LL011 extension).
func isDemoPackage(f *loader.ParsedFile) bool {
	demoNames := map[string]bool{
		"studio":    true,
		"dashboard": true,
		"app":       true,
	}
	if demoNames[f.Pkg] {
		return true
	}

	hasTime := false
	hasStore := false
	for _, path := range f.Imports {
		if path == "time" || strings.HasSuffix(path, "/time") {
			hasTime = true
		}
		if strings.Contains(path, "/store") {
			hasStore = true
		}
	}

	return hasTime && hasStore
}
