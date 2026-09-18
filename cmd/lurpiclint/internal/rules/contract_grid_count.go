package rules

import (
	"go/ast"
	"go/token"
	"strconv"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
)

// gridCountSuspicionThreshold is the GridRows/GridColumns count above which the
// count is suspicious (RX-1 Q6 / FR-6). GridRows/GridColumns declare how many
// tracks a grid has, not their sizes — a count this high usually means the
// author expected the tracks to size to content, which intrinsic tracks do
// automatically, or to compress, which flex tracks never should.
const gridCountSuspicionThreshold = 16

// GridCount flags GridRows/GridColumns counts above the suspicion threshold
// (RX-1 Q6 / FR-6). Counts declare track *count* semantics only; a high count
// on a Card is the A-8 class — many tracks sliced into near-zero bands instead
// of content-sized rows. Set FlexRows/FlexColumns for flex behavior or let
// intrinsic tracks size to content.
type GridCount struct{}

func (r *GridCount) ID() string                     { return "LL036" }
func (r *GridCount) DefaultSeverity() diag.Severity { return diag.SeverityWarn }
func (r *GridCount) Description() string {
	return "GridRows/GridColumns count above 16 is suspicious; counts do not size tracks (RX-1 Q6 / FR-6)"
}

func (r *GridCount) Explain() string {
	return `LL036: a GridRows/GridColumns count above 16 is suspicious.

GridRows and GridColumns declare the number of tracks a grid has — never their
sizes. A count in the hundreds (e.g. a 301-row capability list) historically
sliced the arranged height into near-zero flex bands and silently clipped the
content (the A-8 class). Intrinsic tracks already size to content by default,
and flex is opt-in via FlexRows/FlexColumns; a huge count is therefore a smell.

Fix: use intrinsic tracks (the default) or a table/scroll region for data rows;
if you genuinely need flex distribution, set FlexRows/FlexColumns explicitly
and reconsider why the count is so high.`
}

func (r *GridCount) Check(ctx *Context) []*diag.Diagnostic {
	var diags []*diag.Diagnostic

	for _, f := range ctx.Files {
		if isLayoutOrMarksPackage(f) || isRuntimePackage(f) || isGraphPackage(f) {
			continue
		}
		ast.Inspect(f.AST, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.AssignStmt:
				for _, lhs := range node.Lhs {
					if sel, ok := lhs.(*ast.SelectorExpr); ok && isGridCountField(sel.Sel.Name) {
						if count, ok := gridCountOf(node.Rhs); ok && count > gridCountSuspicionThreshold {
							diags = append(diags, gridCountDiagnostic(r, f, node.Pos(), sel.Sel.Name, count))
						}
					}
				}
			case *ast.CompositeLit:
				for _, elt := range node.Elts {
					kv, ok := elt.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					key, ok := kv.Key.(*ast.Ident)
					if !ok || !isGridCountField(key.Name) {
						continue
					}
					if count, ok := gridCountOf([]ast.Expr{kv.Value}); ok && count > gridCountSuspicionThreshold {
						diags = append(diags, gridCountDiagnostic(r, f, kv.Pos(), key.Name, count))
					}
				}
			}
			return true
		})
	}
	return diags
}

func isGridCountField(name string) bool {
	return name == "GridRows" || name == "GridColumns"
}

// gridCountOf returns the integer literal count when the expression is a
// marks.Const(<int>) call or a bare integer literal.
func gridCountOf(exprs []ast.Expr) (int, bool) {
	for _, e := range exprs {
		if call, ok := e.(*ast.CallExpr); ok {
			if len(call.Args) != 1 {
				continue
			}
			if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.INT {
				if n, err := strconv.Atoi(lit.Value); err == nil {
					return n, true
				}
			}
			continue
		}
		if lit, ok := e.(*ast.BasicLit); ok && lit.Kind == token.INT {
			if n, err := strconv.Atoi(lit.Value); err == nil {
				return n, true
			}
		}
	}
	return 0, false
}

func gridCountDiagnostic(r *GridCount, f *loader.ParsedFile, pos token.Pos, field string, count int) *diag.Diagnostic {
	return &diag.Diagnostic{
		RuleID:   r.ID(),
		Severity: r.DefaultSeverity(),
		Pos:      f.Fset.Position(pos),
		Message:  field + " count " + strconv.Itoa(count) + " is above the suspicion threshold (16); counts do not size tracks — use intrinsic tracks or a table/scroll region",
		Teach: diag.Teaching{
			Did:      "declared a large grid count expecting content-sized rows",
			UseThis:  "rely on intrinsic tracks (the Card default) or use structure.Table/ScrollRegion for data rows; opt into FlexRows/FlexColumns only for true flex fill",
			IndexRef: "structure.Card FlexRows/FlexColumns (RX-1 Q6 / FR-6)",
		},
	}
}

func init() {
	DefaultRegistry.Register(&GridCount{})
}
