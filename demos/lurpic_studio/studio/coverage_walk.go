package studio

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/marks"
)

// walkMarkDescriptors collects the (Family, TypeName) descriptor of every
// reachable marks.Mark in the facet tree rooted at root. It is the shared
// live-tree walk used by the coverage audit (FR-coverage), the demonstration-
// intent review (FR-coverage-distinct), and the inspector pane's mark count.
//
// It walks facet-tree children only: the group-parent container marks
// (Card, ScrollRegion, Tabs) self-project their content without attaching it to
// the facet tree (F-card-content / F-scroll-content), so read-only content
// inside them (e.g. the Capability Index catalog rows) is intentionally outside
// the walk — the same boundary the runtime's projection and hit-testing use.
func walkMarkDescriptors(root facet.FacetImpl) []marks.Descriptor {
	if root == nil || root.Base() == nil {
		return nil
	}
	out := make([]marks.Descriptor, 0, 64)
	stack := []facet.FacetImpl{root}
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if node == nil || node.Base() == nil {
			continue
		}
		if m, ok := node.(marks.Mark); ok {
			out = append(out, m.Descriptor())
		}
		children := node.Base().Children()
		for i := len(children) - 1; i >= 0; i-- {
			if child := children[i]; child != nil && child.Impl() != nil {
				stack = append(stack, child.Impl())
			}
		}
	}
	return out
}

// markDescriptorMultiset counts placed mark descriptors by family and type
// name.
func markDescriptorMultiset(descs []marks.Descriptor) map[string]int {
	out := make(map[string]int, len(descs))
	for _, d := range descs {
		out[d.Family+"/"+d.TypeName]++
	}
	return out
}
