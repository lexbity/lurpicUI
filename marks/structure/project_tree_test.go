package structure

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

// projectTreeCommands projects a facet and its whole subtree in the runtime's
// pre-order paint order (parent chrome first, then each tree child). Composite
// marks whose content is a real tree child (Card, ScrollRegion, List — RX-1
// F-card-content / F-scroll-content) no longer self-project their content, so
// a standalone golden must walk the tree to capture the full rendering. This
// is the shared projection the golden helpers use instead of duplicating a
// one-level child loop.
func projectTreeCommands(root facet.FacetImpl, rt facet.RuntimeServices) gfx.CommandList {
	merged := gfx.CommandList{}
	projectChrome := func(impl facet.FacetImpl) {
		if impl == nil || impl.Base() == nil {
			return
		}
		role := impl.Base().ProjectionRole()
		if role == nil {
			return
		}
		bounds := gfx.Rect{}
		if lr := impl.Base().LayoutRole(); lr != nil {
			bounds = lr.ArrangedBounds
		}
		if cmds := role.Project(facet.ProjectionContext{Runtime: rt, Bounds: bounds, ContentScale: 1}); cmds != nil {
			merged.Commands = append(merged.Commands, cmds.Commands...)
		}
	}
	stack := []facet.FacetImpl{root}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if n == nil || n.Base() == nil {
			continue
		}
		projectChrome(n)
		children := n.Base().Children()
		for i := len(children) - 1; i >= 0; i-- {
			if children[i] != nil && children[i].LayoutRole() != nil {
				stack = append(stack, children[i].Impl())
			}
		}
	}
	return merged
}
