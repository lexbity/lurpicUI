// Package verifylayout provides a runtime layout-tree assertion backed by
// layout.System.  It drives one measure+arrange pass against a window size
// and walks the facet tree checking structural soundness: every rendered
// facet has non-empty bounds, siblings don't overlap (outside sanctioned
// exemptions), and children don't overflow their parent (outside clip-visible
// exemptions).
package verifylayout

import (
	"fmt"
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
)

// Kind classifies a structural-soundness finding.
type Kind string

const (
	KindEmptyBounds         Kind = "empty-bounds"
	KindMeasuredNotArranged Kind = "measured-not-arranged"
	KindSiblingOverlap      Kind = "sibling-overlap"
	KindChildOutOfParent    Kind = "child-out-of-parent"
)

// Finding records one structural-soundness violation.
type Finding struct {
	Kind   Kind
	Type   string // facet type name (Go type or constructed name)
	Field  string // field/receiver name, if discoverable
	Source string // file:line of construction (from Options.SourceOf)
	Detail string // human-readable description
	Hint   string // one-line fix hint
}

// Options controls the assertion behaviour.
type Options struct {
	// Size is the window size for the measure+arrange pass (default 1280×800).
	Size gfx.Size

	// Tolerance is the overlap/out-of-parent slack in pixels (default 0.5).
	Tolerance float32

	// SkipOverlap disables the sibling-overlap check entirely.
	SkipOverlap bool

	// Suppress suppresses specific finding kinds (e.g. for known sanctioned cases).
	Suppress map[Kind]bool

	// SourceOf returns a file:line string for a facet ID.  When set,
	// Finding.Source is populated.  The test typically builds this map
	// at construction time from the builder callsites.
	SourceOf func(facet.FacetID) string
}

func (o *Options) size() gfx.Size {
	if o.Size.W > 0 && o.Size.H > 0 {
		return o.Size
	}
	return gfx.Size{W: 1280, H: 800}
}

func (o *Options) tolerance() float32 {
	if o.Tolerance > 0 {
		return o.Tolerance
	}
	return 0.5
}

func (o *Options) suppressed(k Kind) bool {
	return o != nil && o.Suppress[k]
}

// OverlapExempt is an optional interface that FacetImpl implementations can
// implement to mark themselves as exempt from sibling-overlap checks (e.g. a
// custom stack-like container).
type OverlapExempt interface {
	OverlapExempt() bool
}

// Assert drives one measure+arrange pass over root, walks the tree, and
// reports every structural-soundness violation to tb.  It returns all
// findings (suppressed findings are reported but excluded from the failure).
func Assert(tb testing.TB, root facet.FacetImpl, opts Options) []Finding {
	tb.Helper()
	findings := Check(root, opts)
	for _, f := range findings {
		if opts.suppressed(f.Kind) {
			continue
		}
		src := f.Source
		if src != "" {
			src += ": "
		}
		hint := ""
		if f.Hint != "" {
			hint = "\n  fix: " + f.Hint
		}
		tb.Errorf("verify-layout: [%s] %s%s%s", f.Kind, src, f.Detail, hint)
	}
	return findings
}

// Check runs the same assertion as Assert but returns findings without calling
// tb.Error.  Useful for the CLI front-end.
func Check(root facet.FacetImpl, opts Options) []Finding {
	if root == nil || root.Base() == nil {
		return nil
	}

	sys := layout.NewSystem()
	sys.MarkDirty(root)
	sys.Run(opts.size())

	var findings []Finding
	walkTree(root.Base(), "", &opts, &findings)
	return findings
}

// walkTree traverses the facet tree depth-first and applies the four checks.
func walkTree(f *facet.Facet, path string, opts *Options, findings *[]Finding) {
	if f == nil {
		return
	}

	lr := f.LayoutRole()
	impl := f.Impl()

	// Determine a name for this facet.
	name := typeName(impl)
	if path != "" {
		name = path + "." + name
	}

	// Check 1: empty-bounds — render role with zero-area bounds.
	if !opts.suppressed(KindEmptyBounds) {
		rr := f.RenderRole()
		if rr != nil {
			boundsEmpty := lr == nil || lr.ArrangedBounds.IsEmpty()
			if boundsEmpty {
				*findings = append(*findings, Finding{
					Kind:   KindEmptyBounds,
					Type:   name,
					Source: sourceOf(opts, f.ID()),
					Detail: fmt.Sprintf("%s has a RenderRole but its ArrangedBounds are zero-area", name),
					Hint:   "the facet's parent likely skipped arrangement (no LayoutRole?); ensure the parent registers and invokes a LayoutRole",
				})
			}
		}
	}

	// Check 2: measured-not-arranged — measured but never placed.
	if !opts.suppressed(KindMeasuredNotArranged) {
		if lr != nil && (lr.MeasuredSize.W != 0 || lr.MeasuredSize.H != 0) && lr.ArrangedBounds.IsEmpty() {
			*findings = append(*findings, Finding{
				Kind:   KindMeasuredNotArranged,
				Type:   name,
				Source: sourceOf(opts, f.ID()),
				Detail: fmt.Sprintf("%s was measured (size %v) but never arranged (bounds empty)", name, lr.MeasuredSize),
				Hint:   "the facet was measured by its parent but the parent never called Arrange on it; check the parent's OnArrange",
			})
		}
	}

	// Children.
	children := f.Children()
	if len(children) == 0 {
		return
	}

	// Detect if this parent is a stack-like container.
	isStack := isStackContainer(impl)

	// Check 3: sibling overlap.
	if !opts.suppressed(KindSiblingOverlap) && !opts.SkipOverlap && !isStack {
		for i := 0; i < len(children); i++ {
			for j := i + 1; j < len(children); j++ {
				ci := children[i]
				cj := children[j]
				if ci == nil || cj == nil {
					continue
				}
				lri := ci.LayoutRole()
				lrj := cj.LayoutRole()
				if lri == nil || lrj == nil {
					continue
				}
				if lri.ArrangedBounds.IsEmpty() || lrj.ArrangedBounds.IsEmpty() {
					continue
				}
				if !lri.ArrangedBounds.Intersects(lrj.ArrangedBounds) {
					continue
				}
				// Exempt if either child is an overlay.
				if isOverlay(ci) || isOverlay(cj) {
					continue
				}
				// Exempt if either child implements OverlapExempt.
				if isOverlapExempt(ci.Impl()) || isOverlapExempt(cj.Impl()) {
					continue
				}
				// Exempt if the parent is a recognized stack container.
				if isStack {
					continue
				}

				ni := typeName(ci.Impl())
				nj := typeName(cj.Impl())
				boundsI := lri.ArrangedBounds
				boundsJ := lrj.ArrangedBounds
				intersect := rectIntersect(boundsI, boundsJ)
				*findings = append(*findings, Finding{
					Kind:   KindSiblingOverlap,
					Type:   name,
					Source: sourceOf(opts, f.ID()),
					Detail: fmt.Sprintf("%s and %s overlap by %.0fx%.0fpx", ni, nj, intersect.Width(), intersect.Height()),
					Hint:   "if both should be visible, separate them (e.g. a Tabs body that swaps children)",
				})
			}
		}
	}

	// Check 4: child-out-of-parent.
	if !opts.suppressed(KindChildOutOfParent) {
		tol := opts.tolerance()
		parentBounds := gfx.Rect{}
		if lr != nil {
			parentBounds = lr.ArrangedBounds
		}
		for _, child := range children {
			if child == nil {
				continue
			}
			clr := child.LayoutRole()
			if clr == nil {
				continue
			}
			if clr.ArrangedBounds.IsEmpty() {
				continue
			}
			// Exempt if parent has an explicit OverflowVisible policy
			// (requiring Kind != 0 to distinguish from the zero-value default).
			if lr != nil && lr.Parent.Kind != 0 && lr.Parent.Overflow == facet.OverflowVisible {
				continue
			}
			if !parentBounds.IsEmpty() && childOutOfParent(clr.ArrangedBounds, parentBounds, tol) {
				cn := typeName(child.Impl())
				excess := boundsExcess(clr.ArrangedBounds, parentBounds)
				*findings = append(*findings, Finding{
					Kind:   KindChildOutOfParent,
					Type:   name,
					Source: sourceOf(opts, child.ID()),
					Detail: fmt.Sprintf("%s extends %.0fpx beyond its parent %s", cn, excess, name),
					Hint:   "wrap the column in structure.ScrollRegion or add a Flexible child to absorb overflow",
				})
			}
		}
	}

	// Recurse into children.
	for _, child := range children {
		if child == nil {
			return
		}
		walkTree(child, name, opts, findings)
	}
}

// isStackContainer reports whether impl is a stack-like container whose
// children legitimately overlap.
func isStackContainer(impl facet.FacetImpl) bool {
	if impl == nil {
		return false
	}
	if _, ok := impl.(*layout.StackLayout); ok {
		return true
	}
	return false
}

// isOverlapExempt reports whether impl implements OverlapExempt with a true
// return value.
func isOverlapExempt(impl facet.FacetImpl) bool {
	if impl == nil {
		return false
	}
	ex, ok := impl.(OverlapExempt)
	return ok && ex.OverlapExempt()
}

// isOverlay reports whether a facet is an overlay mark, using the same
// heuristics as LL014/LL021 (presence of an OverlayRole or Open field).
func isOverlay(f *facet.Facet) bool {
	if f == nil {
		return false
	}

	// Check for non-zero layer ZPriority (set by facet.AttachLayer).
	if f.LayerZPriority() > 0 {
		return true
	}

	// Check if the impl has a HitRole with a Dismissal (overlay-like).
	return f.HitRole() != nil
}

// childOutOfParent reports whether childBounds extends beyond parentBounds
// by more than tol on any edge.
func childOutOfParent(child, parent gfx.Rect, tol float32) bool {
	return child.Min.X < parent.Min.X-tol ||
		child.Min.Y < parent.Min.Y-tol ||
		child.Max.X > parent.Max.X+tol ||
		child.Max.Y > parent.Max.Y+tol
}

// boundsExcess returns the maximum pixel by which child exceeds parent on any
// edge, or 0 if it doesn't exceed.
func boundsExcess(child, parent gfx.Rect) float32 {
	excess := float32(0)
	if d := child.Min.X - parent.Min.X; d < 0 {
		excess = max(excess, -d)
	}
	if d := child.Min.Y - parent.Min.Y; d < 0 {
		excess = max(excess, -d)
	}
	if d := child.Max.X - parent.Max.X; d > 0 {
		excess = max(excess, d)
	}
	if d := child.Max.Y - parent.Max.Y; d > 0 {
		excess = max(excess, d)
	}
	return excess
}

// rectIntersect returns the intersection of two rects.
func rectIntersect(a, b gfx.Rect) gfx.Rect {
	minX := max(a.Min.X, b.Min.X)
	minY := max(a.Min.Y, b.Min.Y)
	maxX := min(a.Max.X, b.Max.X)
	maxY := min(a.Max.Y, b.Max.Y)
	if maxX < minX {
		maxX = minX
	}
	if maxY < minY {
		maxY = minY
	}
	return gfx.Rect{Min: gfx.Point{X: minX, Y: minY}, Max: gfx.Point{X: maxX, Y: maxY}}
}

// typeName returns a human-readable name for a FacetImpl.
func typeName(impl facet.FacetImpl) string {
	if impl == nil {
		return "nil"
	}
	s := fmt.Sprintf("%T", impl)
	// Strip package path: keep only the last segment after the last dot or slash.
	if idx := strings.LastIndex(s, "."); idx >= 0 {
		s = s[idx+1:]
	}
	if idx := strings.LastIndex(s, "/"); idx >= 0 {
		s = s[idx+1:]
	}
	// Remove leading * for pointers.
	s = strings.TrimPrefix(s, "*")
	return s
}

// sourceOf returns the source location for a facet ID if SourceOf is set.
func sourceOf(opts *Options, id facet.FacetID) string {
	if opts == nil || opts.SourceOf == nil {
		return ""
	}
	return opts.SourceOf(id)
}

func max(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func min(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}
