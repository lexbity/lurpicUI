package layout

import (
	"sort"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

// System runs measure and arrange passes for marked layout roots.
type System struct {
	dirtyRoots map[facet.FacetID]facet.FacetImpl
}

// NewSystem creates a layout system.
func NewSystem() *System {
	return &System{
		dirtyRoots: make(map[facet.FacetID]facet.FacetImpl),
	}
}

// MarkDirty schedules a facet subtree for a layout pass.
func (s *System) MarkDirty(f facet.FacetImpl) {
	if s == nil || f == nil || f.Base() == nil {
		return
	}
	if s.dirtyRoots == nil {
		s.dirtyRoots = make(map[facet.FacetID]facet.FacetImpl)
	}
	s.dirtyRoots[f.Base().ID()] = f
}

// Run executes layout for all dirty roots against the given window size.
func (s *System) Run(windowSize gfx.Size) {
	if s == nil || len(s.dirtyRoots) == 0 {
		return
	}
	roots := s.selectedRoots()
	for _, root := range roots {
		if root == nil || root.Base() == nil {
			continue
		}
		bounds := gfx.RectFromXYWH(0, 0, windowSize.W, windowSize.H)
		if root.Base().Parent() != nil {
			layoutRole := root.Base().LayoutRole()
			if layoutRole != nil && !layoutRole.ArrangedBounds.IsEmpty() {
				bounds = layoutRole.ArrangedBounds
			}
		}
		measureChild(root, Loose(boundsSize(bounds)))
		arrangeChild(root, bounds)
		clearLayoutDirty(root)
	}
	s.dirtyRoots = make(map[facet.FacetID]facet.FacetImpl)
}

func (s *System) selectedRoots() []facet.FacetImpl {
	if len(s.dirtyRoots) == 0 {
		return nil
	}
	roots := make([]facet.FacetImpl, 0, len(s.dirtyRoots))
	for _, f := range s.dirtyRoots {
		roots = append(roots, f)
	}
	sort.SliceStable(roots, func(i, j int) bool {
		return roots[i].Base().ID() < roots[j].Base().ID()
	})
	filtered := roots[:0]
	for _, f := range roots {
		if !hasDirtyAncestor(f, s.dirtyRoots) {
			filtered = append(filtered, f)
		}
	}
	return filtered
}

func hasDirtyAncestor(f facet.FacetImpl, dirty map[facet.FacetID]facet.FacetImpl) bool {
	if f == nil || f.Base() == nil {
		return false
	}
	for parent := f.Base().Parent(); parent != nil; parent = parent.Parent() {
		if _, ok := dirty[parent.ID()]; ok {
			return true
		}
	}
	return false
}

func clearLayoutDirty(f facet.FacetImpl) {
	if f == nil || f.Base() == nil {
		return
	}
	if flags := f.Base().DirtyFlags(); flags&facet.DirtyLayout != 0 {
		f.Base().ClearDirty(facet.DirtyLayout)
	}
	for _, child := range f.Base().Children() {
		if child != nil {
			clearLayoutDirty(child)
		}
	}
}

func boundsSize(bounds gfx.Rect) gfx.Size {
	return gfx.Size{W: bounds.Width(), H: bounds.Height()}
}

// measureChild runs the measure pass on one child facet.
func measureChild(f facet.FacetImpl, c Constraints) gfx.Size {
	if f == nil || f.Base() == nil {
		return gfx.Size{}
	}
	role := f.Base().LayoutRole()
	if role == nil {
		return gfx.Size{}
	}
	return role.Measure(c.toFacet())
}

// arrangeChild runs the arrange pass on one child facet.
func arrangeChild(f facet.FacetImpl, bounds gfx.Rect) {
	if f == nil || f.Base() == nil {
		return
	}
	role := f.Base().LayoutRole()
	if role == nil {
		return
	}
	role.Arrange(bounds)
}

// deflateConstraints reduces available size by padding.
func deflateConstraints(c Constraints, insets gfx.Insets) Constraints {
	min := gfx.Size{
		W: maxFloat(0, c.MinSize.W-insets.Left-insets.Right),
		H: maxFloat(0, c.MinSize.H-insets.Top-insets.Bottom),
	}
	max := c.MaxSize
	if max.W > 0 {
		max.W = maxFloat(0, max.W-insets.Left-insets.Right)
	}
	if max.H > 0 {
		max.H = maxFloat(0, max.H-insets.Top-insets.Bottom)
	}
	return Constraints{MinSize: min, MaxSize: max}
}

func maxFloat(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
