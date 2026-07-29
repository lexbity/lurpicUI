package classify

import (
	"go/ast"
	"go/token"
	"strings"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/walk"
)

// MaxResolutionDepth is the recursion cap for helper-function inlining used
// by BodyArrangesChildren.
const MaxResolutionDepth = 3

// localIdent returns the local identifier (alias or basename) used for the
// given import-path suffix in the import table.  Returns fallback if no
// entry matches.
func localIdent(imports loader.ImportTable, suffix string, fallback string) string {
	for local, path := range imports {
		if strings.HasSuffix(path, "/"+suffix) || path == suffix {
			return local
		}
	}
	return fallback
}

// gfxIdent returns the local identifier used for the gfx package.
func gfxIdent(imports loader.ImportTable) string {
	return localIdent(imports, "gfx", "gfx")
}

// facetIdent returns the local identifier used for the facet package.
func facetIdent(imports loader.ImportTable) string {
	return localIdent(imports, "facet", "facet")
}

// IsChildArranging reports whether the given LayoutRole composite literal
// arranges multiple child facets — i.e. it reimplements a layout policy
// rather than simply measuring/placing the facet's own content.
//
// A LayoutRole literal is child-arranging if any of its OnArrange or
// OnMeasure function bodies satisfy any of:
//
//  1. Two or more calls to .Arrange, .Measure, or .OnArrange.
//  2. Two or more gfx.RectFromXYWH(...) constructions or gfx.Rect{...} literals.
//  3. Two or more assignments to .ArrangedBounds.
//  4. One Arrange/Measure call combined with at least one RectFromXYWH,
//     Rect literal, or ArrangedBounds assignment.
//  5. A range statement over a child-collection field.
//  6. A call to a same-package helper whose own body satisfies any of the above
//     (recursive, capped at MaxResolutionDepth).
//
// The resolver parameter enables helper-function inlining; pass nil to skip
// cross-function analysis.
func IsChildArranging(lit *ast.CompositeLit, fset *token.FileSet, imports loader.ImportTable, resolver Resolver) bool {
	for _, key := range []string{"OnArrange", "OnMeasure"} {
		val := walk.KeyValue(lit, key)
		if val == nil {
			continue
		}
		body := walk.FuncLitBody(val)
		if body == nil {
			continue
		}
		if BodyArrangesChildren(body, imports, resolver, 0) {
			return true
		}
	}
	return false
}

// BodyArrangesChildren checks whether a function body contains patterns that
// indicate it is arranging child facets.  The function evaluates arrange
// primitives in the direct body, and if a resolver is provided, also
// recursively inlines arrange-primitive counts from resolved same-package
// helpers (capped at MaxResolutionDepth).
//
// depth is the current recursion depth; callers MUST start at 0.
func BodyArrangesChildren(body *ast.BlockStmt, imports loader.ImportTable, resolver Resolver, depth int) bool {
	arrangeCallCount, rectCount, abCount := countBodyWithHelpers(body, imports, resolver, depth)

	if arrangeCallCount >= 2 {
		return true
	}
	if rectCount >= 2 {
		return true
	}
	if abCount >= 2 {
		return true
	}
	if arrangeCallCount >= 1 && (rectCount >= 1 || abCount >= 1) {
		return true
	}
	if walk.HasRangeOverField(body, []string{"children", "Children", "childs", "items", "Items"}) {
		return true
	}
	return false
}

// countBodyWithHelpers returns total arrange-primitive counts for body,
// including counts from all resolved same-package helpers (recursive).
func countBodyWithHelpers(body *ast.BlockStmt, imports loader.ImportTable, resolver Resolver, depth int) (arrangeCount, rectCount, abCount int) {
	if body == nil || depth > MaxResolutionDepth {
		return 0, 0, 0
	}
	gfxID := gfxIdent(imports)

	arrangeCount = countCallsFiltered(body, "Arrange", "Measure", "OnArrange")
	rectCount = walk.CountRectFromXYWH(body, gfxID) + walk.CountRectLiterals(body, gfxID)
	abCount = walk.CountArrangedBoundsAssignments(body)

	if resolver == nil {
		return
	}

	helperCalls := walk.FindCallExprs(body, func(call *ast.CallExpr) bool {
		return funcName(call) != ""
	})
	for _, call := range helperCalls {
		helperBody := resolver.FuncBody(call)
		if helperBody == nil {
			continue
		}
		ha, hr, hb := countBodyWithHelpers(helperBody, imports, resolver, depth+1)
		arrangeCount += ha
		rectCount += hr
		abCount += hb
	}
	return
}

// countCallsFiltered returns the number of CallExpr nodes in root whose
// function name matches any of the given names (checked via final selector).
func countCallsFiltered(root ast.Node, names ...string) int {
	return walk.CountCalls(root, func(call *ast.CallExpr) bool {
		return walk.CallExprIs(call, names...)
	})
}

// funcName extracts the function or method name from a call expression.
// Returns "" for qualified calls (pkg.Func(...)).
func funcName(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		return fun.Sel.Name
	}
	return ""
}
