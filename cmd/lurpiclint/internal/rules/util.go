package rules

import (
	"go/ast"
	"go/token"
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

// isRuntimePackage reports whether the file lives inside the runtime/
// package tree, where direct callback access is legitimate (LL020 gate).
func isRuntimePackage(f *loader.ParsedFile) bool {
	cleanPath := filepath.ToSlash(f.Path)
	return strings.Contains(cleanPath, "/runtime/") ||
		strings.HasPrefix(cleanPath, "runtime/")
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

// isGraphPackage reports whether the file lives inside the graph/ package
// tree, where internal geometry authoring is legitimate.
func isGraphPackage(f *loader.ParsedFile) bool {
	cleanPath := filepath.ToSlash(f.Path)
	return strings.Contains(cleanPath, "/graph/") ||
		strings.HasPrefix(cleanPath, "graph/")
}

// --- Package-local constructor/role scanner (shared with LL019, LL020) -------

// constructorSignals records whether a constructor function body contains
// the three signals the scanner looks for.
type constructorSignals struct {
	HasAddChild          bool
	HasLayoutRoleAddRole bool
	HasRegisterRoles     bool
}

// pkgConstructors returns all constructor function declarations in the
// package that construct the named type.  A function is a constructor if:
//   - it returns *T or T (by name match), or
//   - its name starts with "New"
func pkgConstructors(pkg *loader.Package, typeName string) []*ast.FuncDecl {
	var out []*ast.FuncDecl
	for _, f := range pkg.Files {
		for _, decl := range f.AST.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			if !isConstructorForType(fn, typeName) {
				continue
			}
			out = append(out, fn)
		}
	}
	return out
}

// isConstructorForType reports whether fn is a constructor for typeName.
func isConstructorForType(fn *ast.FuncDecl, typeName string) bool {
	// Match by return type.
	if fn.Type.Results != nil {
		for _, ret := range fn.Type.Results.List {
			if typeRefersTo(ret.Type, typeName) {
				return true
			}
		}
	}
	// Match by New-prefix name.
	return strings.HasPrefix(fn.Name.Name, "New")
}

// typeRefersTo reports whether expr is a reference to typeName (as *T or T).
func typeRefersTo(expr ast.Expr, typeName string) bool {
	if id, ok := expr.(*ast.Ident); ok && id.Name == typeName {
		return true
	}
	if star, ok := expr.(*ast.StarExpr); ok {
		if id, ok := star.X.(*ast.Ident); ok && id.Name == typeName {
			return true
		}
	}
	return false
}

// constructorRoleSignals inspects a constructor function body and returns
// signal booleans for AddChild/AddRole/RegisterRoles calls on the
// constructed facet value.
func constructorRoleSignals(fn *ast.FuncDecl, layoutFields map[string]bool) constructorSignals {
	var sig constructorSignals
	if fn.Body == nil {
		return sig
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		switch sel.Sel.Name {
		case "AddChild", "AddChildRuntime":
			if isOnFacetReceiver(sel.X) {
				sig.HasAddChild = true
			}
		case "AddRole":
			if isOnFacetReceiver(sel.X) && isLayoutRoleAddr(call.Args, layoutFields) {
				sig.HasLayoutRoleAddRole = true
			}
		case "RegisterRoles":
			if isOnRelatedReceiver(sel.X) {
				sig.HasRegisterRoles = true
			}
		}
		return true
	})
	return sig
}

// isOnFacetReceiver reports whether expr is a selector chain ending in .Facet
// (e.g. p.Facet, b.Facet, r.Facet).
func isOnFacetReceiver(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	return sel.Sel.Name == "Facet"
}

// isOnRelatedReceiver reports whether expr is a simple ident or a selector
// chain starting from one — used to detect RegisterRoles() and similar
// calls on the constructed value or its embedded fields.
func isOnRelatedReceiver(expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.Ident:
		return true
	case *ast.SelectorExpr:
		return isOnRelatedReceiver(e.X)
	}
	return false
}

// isLayoutRoleAddr checks whether any argument to the call is an address-of
// expression (&x.layout, &r.Layout, etc.) where the field name is one of the
// known LayoutRole-typed fields.
func isLayoutRoleAddr(args []ast.Expr, layoutFields map[string]bool) bool {
	for _, arg := range args {
		unary, ok := arg.(*ast.UnaryExpr)
		if !ok || unary.Op != token.AND {
			continue
		}
		sel, ok := unary.X.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		if layoutFields[sel.Sel.Name] {
			return true
		}
	}
	return false
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
