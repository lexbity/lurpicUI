package rules

import (
	"go/ast"
	"path/filepath"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
)

// TestIDUniqueness (LL039) flags duplicate author TestID literals within a
// package's non-test files (LDP QP-7 / FR-7). The runtime panics at attach on
// a duplicate live TestID; this rule catches the statically-visible case at
// author time — two composite-literal `TestID:` keys or two `x.TestID = "..."`
// assignments carrying the same literal in one package cannot both attach.
//
// Only literal strings are compared: computed identities are the dynamic
// case, covered by the attach-time panic.
type TestIDUniqueness struct{}

func (r *TestIDUniqueness) ID() string                     { return "LL039" }
func (r *TestIDUniqueness) DefaultSeverity() diag.Severity { return diag.SeverityError }
func (r *TestIDUniqueness) Description() string {
	return "duplicate author TestID literal in one package; author identities must be unique while attached"
}

func (r *TestIDUniqueness) Explain() string {
	return `LL039: a package assigns the same TestID literal more than once.

Author TestIDs are the LDP locator surface: the runtime panics at attach when
two live facets claim the same identity, because a duplicate makes automation
intermittently target the wrong facet. This rule surfaces the statically
visible case — the same string literal used in two TestID assignments within
one package.

Fix: rename one of the identities. Prefer stable, namespaced literals
("shell.cmdk", "e1.controls.opacity").`
}

func (r *TestIDUniqueness) Check(ctx *Context) []*diag.Diagnostic {
	var diags []*diag.Diagnostic
	seen := map[string]string{} // literal → first assignment site (position string)
	reported := map[string]bool{}

	for _, f := range ctx.Files {
		if isFixtureTestFile(f.Path) {
			continue
		}
		pos := f.Fset
		ast.Inspect(f.AST, func(n ast.Node) bool {
			var (
				literal string
				at      ast.Node
			)
			switch v := n.(type) {
			case *ast.KeyValueExpr:
				// Composite literal key: TestID: "shell.cmdk".
				if ident, ok := v.Key.(*ast.Ident); ok && ident.Name == "TestID" {
					if bl, ok := v.Value.(*ast.BasicLit); ok && bl.Kind.String() == "STRING" {
						literal = bl.Value
						at = v
					}
				}
			case *ast.AssignStmt:
				// Assignment: x.TestID = "shell.cmdk".
				if len(v.Lhs) == 1 && len(v.Rhs) == 1 {
					sel, ok := v.Lhs[0].(*ast.SelectorExpr)
					if !ok || sel.Sel.Name != "TestID" {
						return true
					}
					if bl, ok := v.Rhs[0].(*ast.BasicLit); ok && bl.Kind.String() == "STRING" {
						literal = bl.Value
						at = v
					}
				}
			case *ast.CallExpr:
				// Constructor call: SetTestID("shell.cmdk") or
				// b.SetTestID("shell.cmdk") — the forms mark constructors
				// use (bare or receiver-qualified).
				name := ""
				switch fun := v.Fun.(type) {
				case *ast.Ident:
					name = fun.Name
				case *ast.SelectorExpr:
					name = fun.Sel.Name
				}
				if name == "SetTestID" && len(v.Args) == 1 {
					if bl, ok := v.Args[0].(*ast.BasicLit); ok && bl.Kind.String() == "STRING" {
						literal = bl.Value
						at = v
					}
				}
			default:
				return true
			}
			if literal == "" {
				return true
			}
			if first, exists := seen[literal]; exists {
				if !reported[literal] {
					reported[literal] = true
					diags = append(diags, &diag.Diagnostic{
						RuleID:   r.ID(),
						Severity: r.DefaultSeverity(),
						Pos:      pos.Position(at.Pos()),
						Message:  "duplicate TestID literal " + literal + " (first assigned at " + first + "); author identities must be unique",
						Teach: diag.Teaching{
							Did:      "assigned the same TestID literal twice in one package",
							UseThis:  "a unique, namespaced literal per mark (e.g. \"shell.cmdk\")",
							IndexRef: "facet/testid.go",
						},
					})
				}
				return true
			}
			seen[literal] = pos.Position(at.Pos()).String()
			return true
		})
	}
	return diags
}

// isFixtureTestFile reports whether the path is a Go test file (LL039
// deliberately skips tests: fixtures and harnesses legitimately reuse
// literals across packages under test).
func isFixtureTestFile(path string) bool {
	base := filepath.Base(path)
	return len(base) > 8 && base[len(base)-8:] == "_test.go"
}

func init() {
	DefaultRegistry.Register(&TestIDUniqueness{})
}
