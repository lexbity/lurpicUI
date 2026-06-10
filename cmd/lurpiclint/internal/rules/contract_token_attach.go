package rules

import (
	"go/ast"
	"strings"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/walk"
)

// TokenCaptureAtAttach flags theme token capture (calls to DefaultTokens or
// similar) inside OnAttach functions or constructors.  V2 Rule 3 requires
// tokens to be resolved at projection time, not baked during attachment.
//
// Default severity: warn.
type TokenCaptureAtAttach struct{}

func (r *TokenCaptureAtAttach) ID() string                     { return "LL013" }
func (r *TokenCaptureAtAttach) DefaultSeverity() diag.Severity { return diag.SeverityWarn }
func (r *TokenCaptureAtAttach) Description() string {
	return "theme token captured in OnAttach or constructor; resolve tokens at projection time instead"
}

func (r *TokenCaptureAtAttach) Check(ctx *Context) []*diag.Diagnostic {
	var diags []*diag.Diagnostic

	for _, f := range ctx.Files {
		if !fileContainsFacetType(f) {
			continue
		}

		ast.Inspect(f.AST, func(n ast.Node) bool {
			fn, ok := n.(*ast.FuncDecl)
			if !ok || fn.Name == nil || fn.Body == nil {
				return true
			}
			name := fn.Name.Name
			if name != "OnAttach" && !strings.HasPrefix(name, "New") && name != "Init" {
				return true
			}

			ast.Inspect(fn.Body, func(stmt ast.Node) bool {
				assign, ok := stmt.(*ast.AssignStmt)
				if !ok || len(assign.Lhs) == 0 {
					return true
				}

				// Check RHS for a call producing tokens.
				call, ok := assign.Rhs[0].(*ast.CallExpr)
				if !ok {
					return true
				}

				if !isTokenCall(call) {
					return true
				}

				// Check LHS is a field assignment (selector chain).
				if !walk.SelectorChainContains(assign.Lhs[0], "cachedTokens") &&
					!walk.SelectorChainContains(assign.Lhs[0], "tokens") &&
					!walk.SelectorChainContains(assign.Lhs[0], "Tokens") {
					return true
				}

				diags = append(diags, &diag.Diagnostic{
					RuleID:   r.ID(),
					Severity: r.DefaultSeverity(),
					Pos:      f.Fset.Position(call.Pos()),
					Message:  "theme token captured in " + name + "; resolve tokens at projection time instead (V2 Rule 3)",
					Teach: diag.Teaching{
						Did:      "called a theme token function during attachment or construction",
						UseThis:  "resolve tokens lazily at projection time via the runtime context",
						IndexRef: "",
					},
				})
				return true
			})
			return true
		})
	}

	return diags
}

// isTokenCall reports whether call is a function that produces theme tokens.
func isTokenCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	name := sel.Sel.Name
	return strings.Contains(name, "DefaultToken") || strings.Contains(name, "Token") || name == "Tokens"
}

func init() {
	DefaultRegistry.Register(&TokenCaptureAtAttach{})
}
