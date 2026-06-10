package rules

import (
	"go/ast"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/capindex"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/classify"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/walk"
)

// SuggestShapeMatch emits info-level suggestions when a child-arranging
// LayoutRole's structural fingerprint matches a known capability (mark or
// layout container) in the framework's capability index.
//
// Default severity: info (suggestions, not violations).
type SuggestShapeMatch struct{}

func (r *SuggestShapeMatch) ID() string                     { return "LL004" }
func (r *SuggestShapeMatch) DefaultSeverity() diag.Severity { return diag.SeverityInfo }
func (r *SuggestShapeMatch) Description() string {
	return "child-arranging facet matches a known built-in capability; consider using it directly"
}

func (r *SuggestShapeMatch) Check(ctx *Context) []*diag.Diagnostic {
	// Need the capability index.
	idx, ok := ctx.Index.([]capindex.Capability)
	if !ok || len(idx) == 0 {
		return nil
	}

	var diags []*diag.Diagnostic

	for _, f := range ctx.Files {
		fid := facetIdent(f.Imports)

		ast.Inspect(f.AST, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			if !walk.CompositeLitIs(lit, fid, "LayoutRole") {
				return true
			}

			// Only suggest for child-arranging LayoutRoles (LL003 territory).
			if !classify.IsChildArranging(lit, f.Fset, f.Imports) {
				return true
			}

			// Find a matching capability by fingerprint.
			match := findMatchingCapability(idx)
			if match == nil {
				return true
			}

			diags = append(diags, &diag.Diagnostic{
				RuleID:   r.ID(),
				Severity: r.DefaultSeverity(),
				Pos:      f.Fset.Position(lit.Pos()),
				Message: "this child-arranging LayoutRole resembles " + match.Path +
					"; consider using " + match.Constructor + " instead",
				Teach: diag.Teaching{
					Did:      "wrote a custom facet that arranges children",
					UseThis:  match.Path + " (" + match.Constructor + ")",
					IndexRef: match.Path,
				},
			})
			return true
		})
	}

	return diags
}

// findMatchingCapability picks a known capability that is a container (mark
// or layout).  For Phase 10 this uses a simple heuristic: prefer the first
// mark with IsContainer=true, falling back to the first layout container.
func findMatchingCapability(idx []capindex.Capability) *capindex.Capability {
	for i := range idx {
		if idx[i].Kind == capindex.KindMark && idx[i].Fingerprint.IsContainer {
			return &idx[i]
		}
	}
	for i := range idx {
		if idx[i].Kind == capindex.KindLayout {
			return &idx[i]
		}
	}
	return nil
}

func init() {
	DefaultRegistry.Register(&SuggestShapeMatch{})
}
