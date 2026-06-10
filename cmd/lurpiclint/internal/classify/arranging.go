package classify

import (
	"go/ast"
	"go/token"
	"strings"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/walk"
)

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
//  2. Two or more gfx.RectFromXYWH(...) constructions.
//  3. Two or more assignments to .ArrangedBounds.
//  4. One Arrange/Measure call combined with at least one RectFromXYWH
//     or ArrangedBounds assignment.
//  5. A range statement over a child-collection field.
func IsChildArranging(lit *ast.CompositeLit, fset *token.FileSet, imports loader.ImportTable) bool {
	for _, key := range []string{"OnArrange", "OnMeasure"} {
		val := walk.KeyValue(lit, key)
		if val == nil {
			continue
		}
		body := walk.FuncLitBody(val)
		if body == nil {
			continue
		}
		if bodyArrangesChildren(body, imports) {
			return true
		}
	}
	return false
}

// bodyArrangesChildren checks whether a function body contains patterns that
// indicate it is arranging child facets.
func bodyArrangesChildren(body *ast.BlockStmt, imports loader.ImportTable) bool {
	gfxID := gfxIdent(imports)

	arrangeCallCount := walk.CountCalls(body, func(call *ast.CallExpr) bool {
		return walk.CallExprIs(call, "Arrange", "Measure", "OnArrange")
	})
	rectCount := walk.CountRectFromXYWH(body, gfxID)
	abCount := walk.CountArrangedBoundsAssignments(body)

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
