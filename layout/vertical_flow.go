package layout

import "codeburg.org/lexbit/lurpicui/gfx"

// VerticalFlowSize returns the occupied height and maximum width of a vertical
// flow of boxes separated by the supplied gap.
func VerticalFlowSize(sizes []gfx.Size, gap float32) gfx.Size {
	var width float32
	var height float32
	used := false
	for _, size := range sizes {
		if size.W <= 0 || size.H <= 0 {
			continue
		}
		if used {
			height += gap
		}
		height += size.H
		if size.W > width {
			width = size.W
		}
		used = true
	}
	return gfx.Size{W: width, H: height}
}

// ArrangeVerticalFlow lays out a sequence of boxes vertically within bounds.
// Zero-sized entries are skipped. In RTL mode, narrower entries are aligned to
// the right edge of the available content box.
func ArrangeVerticalFlow(bounds gfx.Rect, padY, gap float32, sizes []gfx.Size, rtl bool) []gfx.Rect {
	return ArrangeVerticalFlowAligned(bounds, padY, gap, sizes, rtl, AlignCenter)
}

// ArrangeVerticalFlowAligned lays out a sequence of boxes vertically within
// bounds using the supplied horizontal alignment.
func ArrangeVerticalFlowAligned(bounds gfx.Rect, padY, gap float32, sizes []gfx.Size, rtl bool, align Alignment) []gfx.Rect {
	rects := make([]gfx.Rect, len(sizes))
	type item struct {
		index int
		size  gfx.Size
	}
	items := make([]item, 0, len(sizes))
	for i, size := range sizes {
		if size.W <= 0 || size.H <= 0 {
			continue
		}
		items = append(items, item{index: i, size: size})
	}
	if len(items) == 0 {
		return rects
	}
	content := VerticalFlowSize(sizes, gap)
	contentLeft := bounds.Min.X + maxFloat(0, (bounds.Width()-content.W)*0.5)
	switch align {
	case AlignStart:
		if rtl {
			contentLeft = bounds.Max.X - content.W
		} else {
			contentLeft = bounds.Min.X
		}
	case AlignEnd:
		if rtl {
			contentLeft = bounds.Min.X
		} else {
			contentLeft = bounds.Max.X - content.W
		}
	default:
		if rtl {
			contentLeft = bounds.Max.X - content.W - maxFloat(0, (bounds.Width()-content.W)*0.5)
		}
	}
	y := bounds.Min.Y + padY
	for i, item := range items {
		x := contentLeft
		switch align {
		case AlignStart:
			if rtl {
				x += content.W - item.size.W
			}
		case AlignEnd:
			if !rtl {
				x += content.W - item.size.W
			}
		default:
			x += maxFloat(0, (content.W-item.size.W)*0.5)
		}
		rects[item.index] = gfx.RectFromXYWH(x, y, item.size.W, item.size.H)
		y += item.size.H
		if i < len(items)-1 {
			y += gap
		}
	}
	return rects
}
