package anchor

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

func anchorRect(anchorPt gfx.Point, size gfx.Size, side facet.AnchorSide, gap float32) gfx.Rect {
	origin := gfx.Point{X: anchorPt.X, Y: anchorPt.Y}
	switch side {
	case facet.AnchorAbove:
		origin.X -= size.W / 2
		origin.Y -= size.H + gap
	case facet.AnchorBelow:
		origin.X -= size.W / 2
		origin.Y += gap
	case facet.AnchorLeft:
		origin.X -= size.W + gap
		origin.Y -= size.H / 2
	case facet.AnchorRight:
		origin.X += gap
		origin.Y -= size.H / 2
	case facet.AnchorCenter:
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
	if w > bounds.Width() {
		w = bounds.Width()
	}
	if h > bounds.Height() {
		h = bounds.Height()
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
