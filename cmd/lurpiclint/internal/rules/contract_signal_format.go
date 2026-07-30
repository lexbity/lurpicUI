package rules

import (
	"go/ast"
	"go/token"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
)

// SignalFormat flags signal.Emit calls that pass fmt.Sprintf, fmt.Errorf, or
// string-concatenation expressions as arguments. Signals should carry typed
// payloads, not formatted strings (FR-14 / P12).
type SignalFormat struct{}

func (r *SignalFormat) ID() string                     { return "LL027" }
func (r *SignalFormat) DefaultSeverity() diag.Severity { return diag.SeverityError }
func (r *SignalFormat) Description() string {
	return "string formatting in signal.Emit; use a typed signal payload instead"
}

func (r *SignalFormat) Explain() string {
	return `LL027: signal.Emit must carry typed payloads, not formatted strings.

The typed-signal contract (FR-14 / P12) requires that every signal.Emit call
pass a well-typed value — never a fmt.Sprintf/fmt.Errorf result or a string
concatenation expression.  String-formatted emits bypass the type system and
make consumers dependent on parsing ad-hoc wire formats.

Fix: define a typed payload struct (or use an existing one) and emit that
value directly instead of formatting a string key.

Example — bad:
    s.Activated.Emit(fmt.Sprintf("zoom:%.0f", pct))

Example — good:
    s.Activated.Emit(MarkAction{Key: "zoom", Source: "palette", ZoomPercent: pct})`
}

// exprContainsFormatting reports whether the expression tree rooted at expr
// contains a fmt.Sprintf/fmt.Errorf call or a string-concatenation expression
// with a non-constant operand.
func exprContainsFormatting(expr ast.Expr, imports loader.ImportTable) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		if found {
			return false
		}
		switch e := n.(type) {
		case *ast.CallExpr:
			sel, ok := e.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			id, ok := sel.X.(*ast.Ident)
			if !ok || !isFmtImport(id.Name, imports) {
				return true
			}
			if sel.Sel.Name == "Sprintf" || sel.Sel.Name == "Errorf" {
				found = true
				return false
			}
			return true
		case *ast.BinaryExpr:
			if e.Op == token.ADD {
				if hasStringLitOperand(e) {
					found = true
					return false
				}
			}
			return true
		default:
			return true
		}
	})
	return found
}

// hasStringLitOperand reports whether a binary addition expression tree
// contains a string literal.
func hasStringLitOperand(expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.BasicLit:
		return e.Kind == token.STRING
	case *ast.BinaryExpr:
		if e.Op == token.ADD {
			return hasStringLitOperand(e.X) || hasStringLitOperand(e.Y)
		}
		return false
	case *ast.ParenExpr:
		return hasStringLitOperand(e.X)
	default:
		return false
	}
}

func (r *SignalFormat) Check(ctx *Context) []*diag.Diagnostic {
	var diags []*diag.Diagnostic

	for _, pkg := range ctx.Pkgs {
		for _, pf := range pkg.Files {
			// Skip framework packages (allowlist gate).
			if isLayoutOrMarksPackage(pf) || isRuntimePackage(pf) || isGraphPackage(pf) {
				continue
			}

			ast.Inspect(pf.AST, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}

				// Must be a .Emit(...) call.
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "Emit" {
					return true
				}

				for _, arg := range call.Args {
					if exprContainsFormatting(arg, pf.Imports) {
						diags = append(diags, &diag.Diagnostic{
							RuleID:   r.ID(),
							Severity: r.DefaultSeverity(),
							Pos:      pf.Fset.Position(arg.Pos()),
							Message:  "string formatting in signal.Emit; use a typed signal payload instead",
							Teach: diag.Teaching{
								Did:      "passed a formatted string to signal.Emit",
								UseThis:  "define and emit a typed payload struct",
								IndexRef: "signal.Signal[T] (P12: typed, no string routing)",
							},
						})
					}
				}

				return true
			})
		}
	}

	return diags
}

func init() {
	DefaultRegistry.Register(&SignalFormat{})
}
