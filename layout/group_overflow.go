package layout

import "codeburg.org/lexbit/lurpicui/facet"

// GroupOverflowClipsContent reports whether the overflow policy clips content
// outside the group's arranged bounds.
func GroupOverflowClipsContent(policy facet.OverflowPolicy) bool {
	switch policy {
	case facet.OverflowClip, facet.OverflowScroll:
		return true
	default:
		return false
	}
}
