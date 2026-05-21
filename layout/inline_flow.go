package layout

import "codeburg.org/lexbit/lurpicui/gfx"

// InlineFlowSegment describes one box in a horizontal flow and the gap that
// should follow it.
type InlineFlowSegment struct {
	Size     gfx.Size
	GapAfter float32
}

// InlineFlowSize returns the occupied width and maximum height of a horizontal
// flow of boxes separated by the supplied gap.
func InlineFlowSize(sizes []gfx.Size, gap float32) gfx.Size {
	var width float32
	var height float32
	used := false
	for _, size := range sizes {
		if size.W <= 0 || size.H <= 0 {
			continue
		}
		if used {
			width += gap
		}
		width += size.W
		if size.H > height {
			height = size.H
		}
		used = true
	}
	return gfx.Size{W: width, H: height}
}

// ArrangeInlineFlow lays out a sequence of boxes horizontally within bounds.
// Zero-sized entries are skipped. In RTL mode, the logical order is reversed.
func ArrangeInlineFlow(bounds gfx.Rect, padX, gap float32, sizes []gfx.Size, rtl bool) []gfx.Rect {
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
	content := InlineFlowSize(sizes, gap)
	contentTop := bounds.Min.Y + maxFloat(0, (bounds.Height()-content.H)*0.5)
	if rtl {
		x := bounds.Max.X - padX
		for i := len(items) - 1; i >= 0; i-- {
			size := items[i].size
			x -= size.W
			rects[items[i].index] = gfx.RectFromXYWH(x, contentTop+maxFloat(0, (content.H-size.H)*0.5), size.W, size.H)
			if i > 0 {
				x -= gap
			}
		}
		return rects
	}
	x := bounds.Min.X + padX
	for i, item := range items {
		rects[item.index] = gfx.RectFromXYWH(x, contentTop+maxFloat(0, (content.H-item.size.H)*0.5), item.size.W, item.size.H)
		x += item.size.W
		if i < len(items)-1 {
			x += gap
		}
	}
	return rects
}

// InlineFlowSegmentsSize returns the occupied width and maximum height of a
// horizontal flow of boxes with per-segment trailing gaps.
func InlineFlowSegmentsSize(segments []InlineFlowSegment) gfx.Size {
	items := make([]InlineFlowSegment, 0, len(segments))
	for _, segment := range segments {
		if segment.Size.W <= 0 || segment.Size.H <= 0 {
			continue
		}
		items = append(items, segment)
	}
	var width float32
	var height float32
	for i, segment := range items {
		width += segment.Size.W
		if segment.Size.H > height {
			height = segment.Size.H
		}
		if i < len(items)-1 {
			width += segment.GapAfter
		}
	}
	return gfx.Size{W: width, H: height}
}

// ArrangeInlineFlowSegments lays out a sequence of boxes horizontally within
// bounds using per-segment trailing gaps. Zero-sized entries are skipped. In
// RTL mode, the logical order is reversed.
func ArrangeInlineFlowSegments(bounds gfx.Rect, padX float32, segments []InlineFlowSegment, rtl bool) []gfx.Rect {
	rects := make([]gfx.Rect, len(segments))
	type item struct {
		index   int
		segment InlineFlowSegment
	}
	items := make([]item, 0, len(segments))
	for i, segment := range segments {
		if segment.Size.W <= 0 || segment.Size.H <= 0 {
			continue
		}
		items = append(items, item{index: i, segment: segment})
	}
	if len(items) == 0 {
		return rects
	}
	content := InlineFlowSegmentsSize(segments)
	contentTop := bounds.Min.Y + maxFloat(0, (bounds.Height()-content.H)*0.5)
	if rtl {
		x := bounds.Max.X - padX
		for i := len(items) - 1; i >= 0; i-- {
			segment := items[i].segment
			x -= segment.Size.W
			rects[items[i].index] = gfx.RectFromXYWH(x, contentTop+maxFloat(0, (content.H-segment.Size.H)*0.5), segment.Size.W, segment.Size.H)
			if i > 0 {
				x -= items[i-1].segment.GapAfter
			}
		}
		return rects
	}
	x := bounds.Min.X + padX
	for i, item := range items {
		rects[item.index] = gfx.RectFromXYWH(x, contentTop+maxFloat(0, (content.H-item.segment.Size.H)*0.5), item.segment.Size.W, item.segment.Size.H)
		x += item.segment.Size.W
		if i < len(items)-1 {
			x += item.segment.GapAfter
		}
	}
	return rects
}
