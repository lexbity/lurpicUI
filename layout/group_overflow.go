package layout

import "codeburg.org/lexbit/lurpicui/facet"

// GroupOverflowClipsContent reports whether the overflow policy clips content
// outside the group's arranged bounds. Scroll and Clip clip; Grow does not
// (it reports min-content upward and is clamped at arrange instead, RX-1 Q5).
func GroupOverflowClipsContent(policy facet.OverflowPolicy) bool {
	switch policy {
	case facet.OverflowScroll, facet.OverflowClip:
		return true
	default:
		return false
	}
}
