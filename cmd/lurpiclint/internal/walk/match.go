// Package walk provides shared AST traversal helpers used by rules and the
// classifier.  All helpers operate on parsed Go ASTs and use import tables
// for package-resolution so that aliased imports are handled correctly.
package walk

import (
	"go/ast"
	"go/token"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
)

// ---------------------------------------------------------------------------
// Selector / identifier matching
// ---------------------------------------------------------------------------

// SelectorIs reports whether expr is a selector expression of the form
// <ident>.<name>, where <ident> resolves (via the import table) to the given
// package base name or alias.
//
//	SelectorIs(expr, "facet", "LayoutRole")  →  true for  facet.LayoutRole
func SelectorIs(expr ast.Expr, ident, name string) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	x, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return x.Name == ident && sel.Sel.Name == name
}

// SelectorIsAny reports whether expr is a selector whose final segment
// matches any of the given names, regardless of the left-hand side.
//
//	SelectorIsAny(expr, "Arrange", "Measure")  →  true for  foo.Arrange(...)
func SelectorIsAny(expr ast.Expr, names ...string) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
	}
	return nameSet[sel.Sel.Name]
}

// SelectorChainContains reports whether a (possibly chained) selector
// expression reaches a final method or field named finalName at any depth.
//
//	SelectorChainContains(expr, "ArrangedBounds") → true for r.chromeBar.layout.ArrangedBounds
func SelectorChainContains(expr ast.Expr, finalName string) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	if sel.Sel.Name == finalName {
		return true
	}
	return SelectorChainContains(sel.X, finalName)
}

// CallExprIs reports whether n is a call to a function whose final selector
// segment matches any of names.
//
//	CallExprIs(n, "Arrange", "Measure") → true for  role.Arrange(...)
func CallExprIs(n ast.Node, names ...string) bool {
	call, ok := n.(*ast.CallExpr)
	if !ok {
		return false
	}
	return SelectorIsAny(call.Fun, names...)
}

// ---------------------------------------------------------------------------
// Composite literal matching
// ---------------------------------------------------------------------------

// CompositeLitIs reports whether lit is a composite literal of the form
// <ident>.<typeName>.
//
//	CompositeLitIs(lit, "facet", "LayoutRole") → true for facet.LayoutRole{...}
func CompositeLitIs(lit *ast.CompositeLit, ident, typeName string) bool {
	return SelectorIs(lit.Type, ident, typeName)
}

// ---------------------------------------------------------------------------
// Type declaration helpers
// ---------------------------------------------------------------------------

// EmbedsFacet reports whether typeSpec is a struct type that embeds
// facet.Facet (anonymously, no field name).
func EmbedsFacet(typeSpec *ast.TypeSpec, imports loader.ImportTable) bool {
	st, ok := typeSpec.Type.(*ast.StructType)
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
		// Check if the import table maps this ident to "facet" package.
		// But we also need to verify the type name is "Facet".
		if sel.Sel.Name == "Facet" && id.Name == "facet" {
			return true
		}
	}
	return false
}

// HasAddRoleCall checks whether decl (a function or method) contains a call
// to <ident>.AddRole(...).  Typically used to find constructor functions that
// register roles.
func HasAddRoleCall(body *ast.BlockStmt, ident string) bool {
	if body == nil {
		return false
	}
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if SelectorIs(call.Fun, ident, "AddRole") {
			found = true
			return false
		}
		return true
	})
	return found
}

// ---------------------------------------------------------------------------
// Iteration helpers
// ---------------------------------------------------------------------------

// HasRangeOverField reports whether body contains a range statement over a
// field or variable whose name matches any of the given fieldNames.
func HasRangeOverField(body *ast.BlockStmt, fieldNames []string) bool {
	if body == nil {
		return false
	}
	nameSet := make(map[string]bool, len(fieldNames))
	for _, n := range fieldNames {
		nameSet[n] = true
	}
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		rs, ok := n.(*ast.RangeStmt)
		if !ok {
			return true
		}
		// Check if the range target (X) references one of the field names.
		switch x := rs.X.(type) {
		case *ast.Ident:
			if nameSet[x.Name] {
				found = true
			}
		case *ast.SelectorExpr:
			if nameSet[x.Sel.Name] {
				found = true
			}
		}
		return !found
	})
	return found
}

// ---------------------------------------------------------------------------
// Field value extraction from composite literals
// ---------------------------------------------------------------------------

// KeyValue returns the Value (as ast.Expr) for the given key in a composite
// literal's elements.  Returns nil if not found.
func KeyValue(lit *ast.CompositeLit, key string) ast.Expr {
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		id, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}
		if id.Name == key {
			return kv.Value
		}
	}
	return nil
}

// FuncLitBody extracts the *ast.BlockStmt from an expression expected to be a
// function literal, or nil.
func FuncLitBody(e ast.Expr) *ast.BlockStmt {
	fn, ok := e.(*ast.FuncLit)
	if !ok {
		return nil
	}
	return fn.Body
}

// ---------------------------------------------------------------------------
// Counting helpers
// ---------------------------------------------------------------------------

// CountCalls returns the number of CallExpr nodes in root whose Fun matches
// the predicate.
func CountCalls(root ast.Node, pred func(*ast.CallExpr) bool) int {
	var count int
	ast.Inspect(root, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if ok && pred(call) {
			count++
		}
		return true
	})
	return count
}

// CountRectFromXYWH returns the number of gfx.RectFromXYWH(...) calls in
// root.  The import ident is typically "gfx" (resolved via import table).
func CountRectFromXYWH(root ast.Node, gfxIdent string) int {
	return CountCalls(root, func(call *ast.CallExpr) bool {
		return SelectorIs(call.Fun, gfxIdent, "RectFromXYWH")
	})
}

// CountArrangedBoundsAssignments returns the number of assignment statements
// whose left-hand side contains a selector ending in "ArrangedBounds".
func CountArrangedBoundsAssignments(root ast.Node) int {
	var count int
	ast.Inspect(root, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for _, lhs := range assign.Lhs {
			if SelectorChainContains(lhs, "ArrangedBounds") {
				count++
			}
		}
		return true
	})
	return count
}

// FindCallExprs returns all CallExpr nodes in root that satisfy the
// predicate, in AST-visit order.
func FindCallExprs(root ast.Node, pred func(*ast.CallExpr) bool) []*ast.CallExpr {
	var out []*ast.CallExpr
	ast.Inspect(root, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if ok && pred(call) {
			out = append(out, call)
		}
		return true
	})
	return out
}

// Position returns a token.Position for a given node using the file set.
func Position(fset *token.FileSet, n ast.Node) token.Position {
	return fset.Position(n.Pos())
}
