package anchor

import (
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
)

// Policy places children relative to exported anchor positions.
type Policy struct{}

// New constructs an anchor-placement policy.
func New() *Policy {
	return &Policy{}
}

// Measure always returns zero size.
func (p *Policy) Measure(children []layout.ChildNode, constraints gfx.Size) gfx.Size {
	return gfx.Size{}
}

// Arrange positions each child relative to its referenced anchor.
func (p *Policy) Arrange(children []layout.ChildNode, layer layout.ResolvedLayer) {
	if p == nil || len(children) == 0 {
		return
	}
	bounds := layer.Bounds
	allowOverflow := layer.CoordLimits.AllowOverflow
	cache := layer.AnchorCache
	for i := range children {
		child := children[i]
		rect := gfx.Rect{}
		if cache == nil {
			children[i].SetArrangedBounds(rect)
			continue
		}
		anchorPt, ok := cache.Get(child.Attachment.Placement.AnchorRef)
		if !ok {
			children[i].SetArrangedBounds(rect)
			continue
		}
		rect = anchorRect(anchorPt, child.IntrinsicSize, child.Attachment.Placement.AnchorSide, child.Attachment.Placement.AnchorGap)
		if !allowOverflow {
			rect = clampToBounds(rect, bounds)
		}
		children[i].SetArrangedBounds(rect)
	}
}

func anchorRect(anchorPt gfx.Point, size gfx.Size, side layout.AnchorSide, gap float32) gfx.Rect {
	origin := gfx.Point{X: anchorPt.X, Y: anchorPt.Y}
	switch side {
	case layout.AnchorAbove:
		origin.X -= size.W / 2
		origin.Y -= size.H + gap
	case layout.AnchorBelow:
		origin.X -= size.W / 2
		origin.Y += gap
	case layout.AnchorLeft:
		origin.X -= size.W + gap
		origin.Y -= size.H / 2
	case layout.AnchorRight:
		origin.X += gap
		origin.Y -= size.H / 2
	case layout.AnchorCenter:
		origin.X -= size.W / 2
		origin.Y -= size.H / 2
	default:
		origin.X -= size.W / 2
		origin.Y -= size.H + gap
	}
	return gfx.RectFromXYWH(origin.X, origin.Y, size.W, size.H)
}

func clampToBounds(rect, bounds gfx.Rect) gfx.Rect {
	if rect.IsEmpty() || bounds.IsEmpty() {
		return rect
	}
	w := rect.Width()
	h := rect.Height()
	if w > bounds.Width() || h > bounds.Height() {
		return gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, w, h)
	}
	x := rect.Min.X
	y := rect.Min.Y
	if x < bounds.Min.X {
		x = bounds.Min.X
	}
	if y < bounds.Min.Y {
		y = bounds.Min.Y
	}
	if x+w > bounds.Max.X {
		x = bounds.Max.X - w
	}
	if y+h > bounds.Max.Y {
		y = bounds.Max.Y - h
	}
	return gfx.RectFromXYWH(x, y, w, h)
}
