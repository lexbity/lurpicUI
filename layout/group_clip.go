package layout

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

// GroupClipsContent reports whether the group's own contract establishes an
// additional clip volume beyond the inherited parent clip.
func GroupClipsContent(parent facet.GroupParentContract) bool {
	switch parent.Clipping {
	case facet.GroupClipVisible:
		return false
	case facet.GroupClipBounds:
		return true
	case facet.GroupClipInherit:
		return GroupOverflowClipsContent(parent.Overflow)
	default:
		return GroupOverflowClipsContent(parent.Overflow)
	}
}

// GroupClipRect returns the clip rect introduced by the group's own contract.
// The returned rect is in the same coordinate space as the supplied bounds.
func GroupClipRect(bounds gfx.Rect, parent facet.GroupParentContract) (gfx.Rect, bool) {
	if !GroupClipsContent(parent) || bounds.IsEmpty() {
		return gfx.Rect{}, false
	}
	return bounds, true
}

// IntersectClipRects intersects two clip rects and reports whether the result
// is non-empty. A zero-value base means the next rect becomes the effective clip.
func IntersectClipRects(base gfx.Rect, hasBase bool, next gfx.Rect) (gfx.Rect, bool) {
	if next.IsEmpty() {
		return base, hasBase
	}
	if !hasBase || base.IsEmpty() {
		return next, true
	}
	minX := base.Min.X
	if next.Min.X > minX {
		minX = next.Min.X
	}
	minY := base.Min.Y
	if next.Min.Y > minY {
		minY = next.Min.Y
	}
	maxX := base.Max.X
	if next.Max.X < maxX {
		maxX = next.Max.X
	}
	maxY := base.Max.Y
	if next.Max.Y < maxY {
		maxY = next.Max.Y
	}
	clip := gfx.RectFromXYWH(minX, minY, maxX-minX, maxY-minY)
	if clip.IsEmpty() {
		return gfx.Rect{}, false
	}
	return clip, true
}
