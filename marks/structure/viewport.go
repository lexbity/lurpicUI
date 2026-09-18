package structure

import (
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/mathutil"
)

// ScrollViewport is the shared scroll-geometry helper for the structure marks
// (scroll_region, table; RX-1 FR-6). It owns the scroll-offset clamping and
// the track/thumb geometry for both axes so a scrollbar affordance is
// implemented once. The marks keep their own offset state and cached geometry
// but delegate the math here.
type ScrollViewport struct {
	// Content is the full scrollable content size.
	Content gfx.Size
	// View is the arranged viewport rect.
	View gfx.Rect
	// Track is the scrollbar thickness.
	Track float32
}

// MaxX returns the maximum horizontal scroll offset.
func (v ScrollViewport) MaxX() float32 {
	return mathutil.Max(0, v.Content.W-v.View.Width())
}

// MaxY returns the maximum vertical scroll offset.
func (v ScrollViewport) MaxY() float32 {
	return mathutil.Max(0, v.Content.H-v.View.Height())
}

// Clamp clamps a scroll offset to [0, max] on each axis.
func (v ScrollViewport) Clamp(offset gfx.Point) gfx.Point {
	return gfx.Point{
		X: clampFloat(offset.X, 0, v.MaxX()),
		Y: clampFloat(offset.Y, 0, v.MaxY()),
	}
}

// Vertical returns the vertical track and thumb rects for the given offset.
func (v ScrollViewport) Vertical(offset gfx.Point) (track, thumb gfx.Rect) {
	if v.MaxY() <= 0 {
		return gfx.Rect{}, gfx.Rect{}
	}
	trackHeight := v.View.Height()
	if v.MaxX() > 0 {
		trackHeight -= v.Track
	}
	if trackHeight < 0 {
		trackHeight = 0
	}
	track = gfx.RectFromXYWH(v.View.Max.X-v.Track, v.View.Min.Y, v.Track, trackHeight)
	thumbHeight := mathutil.Max(v.Track*2, trackHeight*(v.View.Height()/mathutil.Max(1, v.Content.H)))
	if thumbHeight > trackHeight {
		thumbHeight = trackHeight
	}
	maxOffset := mathutil.Max(1, v.MaxY())
	thumbY := v.View.Min.Y + (offset.Y/maxOffset)*(trackHeight-thumbHeight)
	thumb = gfx.RectFromXYWH(v.View.Max.X-v.Track, thumbY, v.Track, thumbHeight)
	return track, thumb
}

// Horizontal returns the horizontal track and thumb rects for the given offset.
func (v ScrollViewport) Horizontal(offset gfx.Point) (track, thumb gfx.Rect) {
	if v.MaxX() <= 0 {
		return gfx.Rect{}, gfx.Rect{}
	}
	trackWidth := v.View.Width()
	if v.MaxY() > 0 {
		trackWidth -= v.Track
	}
	if trackWidth < 0 {
		trackWidth = 0
	}
	track = gfx.RectFromXYWH(v.View.Min.X, v.View.Max.Y-v.Track, trackWidth, v.Track)
	thumbWidth := mathutil.Max(v.Track*2, trackWidth*(v.View.Width()/mathutil.Max(1, v.Content.W)))
	if thumbWidth > trackWidth {
		thumbWidth = trackWidth
	}
	maxOffset := mathutil.Max(1, v.MaxX())
	thumbX := v.View.Min.X + (offset.X/maxOffset)*(trackWidth-thumbWidth)
	thumb = gfx.RectFromXYWH(thumbX, v.View.Max.Y-v.Track, thumbWidth, v.Track)
	return track, thumb
}

// DragTo maps a drag pointer position along a scrollbar track to a new offset
// for the given axis.
func (v ScrollViewport) DragTo(offset gfx.Point, horizontal bool, p gfx.Point, track, thumb gfx.Rect) gfx.Point {
	if track.IsEmpty() || thumb.IsEmpty() {
		return offset
	}
	if horizontal {
		maxOffset := v.MaxX()
		if maxOffset <= 0 {
			return offset
		}
		trackSpan := mathutil.Max(1, track.Width()-thumb.Width())
		pos := p.X - track.Min.X - thumb.Width()*0.5
		offset.X = clampFloat((pos/trackSpan)*maxOffset, 0, maxOffset)
		return offset
	}
	maxOffset := v.MaxY()
	if maxOffset <= 0 {
		return offset
	}
	trackSpan := mathutil.Max(1, track.Height()-thumb.Height())
	pos := p.Y - track.Min.Y - thumb.Height()*0.5
	offset.Y = clampFloat((pos/trackSpan)*maxOffset, 0, maxOffset)
	return offset
}
