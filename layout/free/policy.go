package free

import (
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
)

// Policy places children at absolute positions anchored to the layer bounds.
type Policy struct{}

// New constructs a free-placement policy.
func New() *Policy {
	return &Policy{}
}

// Measure always returns zero size.
func (p *Policy) Measure(children []layout.ChildNode, constraints gfx.Size) gfx.Size {
	return gfx.Size{}
}

// Arrange positions each child relative to its declared free anchor.
func (p *Policy) Arrange(children []layout.ChildNode, layer layout.ResolvedLayer) {
	if p == nil || len(children) == 0 {
		return
	}
	allowOverflow := layer.CoordLimits.AllowOverflow
	bounds := layer.Bounds
	for i := range children {
		child := children[i]
		rect := place(bounds, child.IntrinsicSize, child.Attachment.Placement.FreeAnchor, child.Attachment.Placement.Offset)
		if !allowOverflow {
			rect = clampToBounds(rect, bounds)
		}
		children[i].SetArrangedBounds(rect)
	}
}

func place(bounds gfx.Rect, size gfx.Size, anchor layout.FreeAnchor, offset gfx.Point) gfx.Rect {
	anchorPt := anchorPoint(bounds, anchor)
	origin := gfx.Point{X: anchorPt.X + offset.X, Y: anchorPt.Y + offset.Y}
	min := origin
	switch anchor {
	case layout.FreeTopLeft, layout.FreeTopCenter, layout.FreeTopRight:
	case layout.FreeCenterLeft, layout.FreeCenter, layout.FreeCenterRight:
		min.Y -= size.H / 2
	case layout.FreeBottomLeft, layout.FreeBottomCenter, layout.FreeBottomRight:
		min.Y -= size.H
	}
	switch anchor {
	case layout.FreeTopLeft, layout.FreeCenterLeft, layout.FreeBottomLeft:
	case layout.FreeTopCenter, layout.FreeCenter, layout.FreeBottomCenter:
		min.X -= size.W / 2
	case layout.FreeTopRight, layout.FreeCenterRight, layout.FreeBottomRight:
		min.X -= size.W
	}
	return gfx.RectFromXYWH(min.X, min.Y, size.W, size.H)
}

func anchorPoint(bounds gfx.Rect, anchor layout.FreeAnchor) gfx.Point {
	midX := (bounds.Min.X + bounds.Max.X) / 2
	midY := (bounds.Min.Y + bounds.Max.Y) / 2
	switch anchor {
	case layout.FreeTopLeft:
		return gfx.Point{X: bounds.Min.X, Y: bounds.Min.Y}
	case layout.FreeTopCenter:
		return gfx.Point{X: midX, Y: bounds.Min.Y}
	case layout.FreeTopRight:
		return gfx.Point{X: bounds.Max.X, Y: bounds.Min.Y}
	case layout.FreeCenterLeft:
		return gfx.Point{X: bounds.Min.X, Y: midY}
	case layout.FreeCenter:
		return gfx.Point{X: midX, Y: midY}
	case layout.FreeCenterRight:
		return gfx.Point{X: bounds.Max.X, Y: midY}
	case layout.FreeBottomLeft:
		return gfx.Point{X: bounds.Min.X, Y: bounds.Max.Y}
	case layout.FreeBottomCenter:
		return gfx.Point{X: midX, Y: bounds.Max.Y}
	case layout.FreeBottomRight:
		return gfx.Point{X: bounds.Max.X, Y: bounds.Max.Y}
	default:
		return gfx.Point{X: bounds.Min.X, Y: bounds.Min.Y}
	}
}

func clampToBounds(rect, bounds gfx.Rect) gfx.Rect {
	if rect.Width() > bounds.Width() || rect.Height() > bounds.Height() {
		return gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, rect.Width(), rect.Height())
	}
	x := rect.Min.X
	y := rect.Min.Y
	if x < bounds.Min.X {
		x = bounds.Min.X
	}
	if y < bounds.Min.Y {
		y = bounds.Min.Y
	}
	if x+rect.Width() > bounds.Max.X {
		x = bounds.Max.X - rect.Width()
	}
	if y+rect.Height() > bounds.Max.Y {
		y = bounds.Max.Y - rect.Height()
	}
	return gfx.RectFromXYWH(x, y, rect.Width(), rect.Height())
}
