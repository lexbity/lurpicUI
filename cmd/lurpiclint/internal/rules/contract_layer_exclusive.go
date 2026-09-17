package rules

import (
	"go/ast"
	"go/token"
	"strings"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
)

// LayerExclusive flags layer-attached children that are ALSO group-arranged.
// RX-1 Q4 grants a layer facet exclusivity: a child mounted via AttachLayer is
// a tree child but must never appear in the host's group-measure/arrange set
// (its Children() list). A child listed there gets arranged by the host's
// group policy AND by the layer system — a double arrangement.
//
// Detection: for each AttachLayer(host, child, ...) call, check whether the
// child identifier also appears in a Children() method of the host's type in
// the same file. Framework packages (marks/runtime/layout/graph) are skipped,
// matching the sibling-overlay rule's scope.
//
// Default severity: error.
type LayerExclusive struct{}

func (r *LayerExclusive) ID() string                     { return "LL034" }
func (r *LayerExclusive) DefaultSeverity() diag.Severity { return diag.SeverityError }
func (r *LayerExclusive) Description() string {
	return "layer-attached facet must not also be a group-arranged child (RX-1 Q4 exclusivity)"
}

func (r *LayerExclusive) Check(ctx *Context) []*diag.Diagnostic {
	var diags []*diag.Diagnostic

	for _, f := range ctx.Files {
		if isLayoutOrMarksPackage(f) || isRuntimePackage(f) || isGraphPackage(f) {
			continue
		}
		if !fileContainsFacetType(f) {
			continue
		}

		type attach struct {
			pos   token.Pos
			child string
		}
		var attaches []attach
		ast.Inspect(f.AST, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != StrAttachLayer {
				return true
			}
			if len(call.Args) < 2 {
				return true
			}
			child := exprChain(call.Args[1])
			if child == "" {
				return true
			}
			attaches = append(attaches, attach{
				pos:   call.Pos(),
				child: child,
			})
			return true
		})
		if len(attaches) == 0 {
			continue
		}

		// Collect every selector chain referenced inside Children() bodies
		// (e.g. "h.content.Base") so a layer child (e.g. "h.overlay") that is
		// ALSO listed as a group child is detected as the prefix of a chain.
		type childrenMethod struct {
			chains map[string]bool
		}
		var children []childrenMethod
		ast.Inspect(f.AST, func(n ast.Node) bool {
			fn, ok := n.(*ast.FuncDecl)
			if !ok || fn.Name.Name != "Children" {
				return true
			}
			if fn.Body == nil {
				return true
			}
			chains := map[string]bool{}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if sel, ok := n.(*ast.SelectorExpr); ok {
					if chain := exprChain(sel); chain != "" {
						chains[chain] = true
					}
				}
				return true
			})
			children = append(children, childrenMethod{chains: chains})
			return true
		})
		if len(children) == 0 {
			continue
		}

		for _, a := range attaches {
			for _, m := range children {
				if chainReferenced(m.chains, a.child) {
					diags = append(diags, &diag.Diagnostic{
						RuleID:   r.ID(),
						Severity: r.DefaultSeverity(),
						Pos:      f.Fset.Position(a.pos),
						Message:  "layer-attached child is also a group-arranged child in Children(); a layer facet is exclusively owned by the layer system (RX-1 Q4)",
						Teach: diag.Teaching{
							Did:      "listed a layer-attached facet in Children()",
							UseThis:  "leave the layer facet out of Children() so the layer system exclusively arranges it",
							IndexRef: "facet.AttachLayer",
						},
					})
					break
				}
			}
		}
	}

	return diags
}

// ProjectPurity flags store writes inside OnProject closures. Projection is a
// read-only pass: writing stores from OnProject breaks the frame's derived
// flush ordering and can produce feedback loops.
//
// Default severity: warn (advisory).
type ProjectPurity struct{}

func (r *ProjectPurity) ID() string                     { return "LL035" }
func (r *ProjectPurity) DefaultSeverity() diag.Severity { return diag.SeverityWarn }
func (r *ProjectPurity) Description() string {
	return "OnProject closures must not write stores; projection is read-only"
}

func (r *ProjectPurity) Check(ctx *Context) []*diag.Diagnostic {
	var diags []*diag.Diagnostic

	for _, f := range ctx.Files {
		if !fileContainsFacetType(f) {
			continue
		}
		ast.Inspect(f.AST, func(n ast.Node) bool {
			assign, ok := n.(*ast.AssignStmt)
			if !ok {
				return true
			}
			if len(assign.Lhs) != 1 {
				return true
			}
			lhs, ok := assign.Lhs[0].(*ast.SelectorExpr)
			if !ok || lhs.Sel == nil || lhs.Sel.Name != "OnProject" {
				return true
			}
			if _, ok := assign.Rhs[0].(*ast.FuncLit); !ok {
				return true
			}
			ast.Inspect(assign.Rhs[0], func(inner ast.Node) bool {
				call, ok := inner.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel == nil {
					return true
				}
				switch sel.Sel.Name {
				case "Set", "Update", "Delete", "Append", "Insert", "Remove", "Write":
					diags = append(diags, &diag.Diagnostic{
						RuleID:   r.ID(),
						Severity: r.DefaultSeverity(),
						Pos:      f.Fset.Position(call.Pos()),
						Message:  "OnProject closure writes a store via " + sel.Sel.Name + "; projection must be read-only",
						Teach: diag.Teaching{
							Did:      "wrote a store inside OnProject",
							UseThis:  "move the write out of projection (into a handler or a store subscription)",
							IndexRef: "OnProject",
						},
					})
				}
				return true
			})
			return true
		})
	}

	return diags
}

func init() {
	DefaultRegistry.Register(&LayerExclusive{})
	DefaultRegistry.Register(&ProjectPurity{})
}

// exprChain returns the dotted selector chain of an expression: "p.surface" for
// p.surface, "p" for a bare ident.
func exprChain(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		base := exprChain(t.X)
		if base == "" {
			return ""
		}
		return base + "." + t.Sel.Name
	case *ast.ParenExpr:
		return exprChain(t.X)
	case *ast.StarExpr:
		return exprChain(t.X)
	default:
		return ""
	}
}

// chainReferenced reports whether any recorded chain starts with the layer
// child's chain (a layer child referenced in Children() is a prefix of a
// selector chain such as child.Base().ID()).
func chainReferenced(chains map[string]bool, child string) bool {
	for c := range chains {
		if c == child || strings.HasPrefix(c, child+".") {
			return true
		}
	}
	return false
}
