package gfx

import (
	"iter"

	"codeburg.org/lexbit/lurpicui/text"
)

// Values is an immutable view of value elements. It deliberately exposes no
// slice: changing a returned element cannot change its backing command page.
type Values[T any] struct{ values []T }

func (v Values[T]) Len() int   { return len(v.values) }
func (v Values[T]) At(i int) T { return v.values[i] }
func (v Values[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		for _, value := range v.values {
			if !yield(value) {
				return
			}
		}
	}
}

// BrushView contains scalar brush parameters and immutable gradient stops.
type BrushView struct {
	Kind                       BrushKind
	Color                      Color
	GradientStart, GradientEnd Point
	GradientStops              Values[GradientStop]
}

func ReadBrush(b Brush) BrushView {
	return BrushView{b.Kind, b.Color, b.GradientStart, b.GradientEnd, Values[GradientStop]{b.GradientStops}}
}

// PathView is the immutable form consumed by rasterizers and encoders.
type PathView struct{ Segments Values[PathSegment] }

func ReadPath(p Path) PathView { return PathView{Values[PathSegment]{p.Segments}} }

// CommandView separates immutable payload access from scalar command values.
// Scalars contains no writable slices. Views obtained from a sealed page stay
// valid only while the consumer owns its frame lease.
type CommandView struct {
	scalars Command
	payload Command
}

// ReadCommand observes an authored command for synchronous encoding. Sealed
// pages construct the same view over their independently owned payloads.
func ReadCommand(command Command) CommandView {
	v := CommandView{scalars: command, payload: command}
	switch c := command.(type) {
	case FillRect:
		c.Brush.GradientStops = nil
		v.scalars = c
	case StrokeRect:
		c.Brush.GradientStops = nil
		c.Stroke.Dash = nil
		v.scalars = c
	case FillPath:
		c.Path.Segments = nil
		c.Brush.GradientStops = nil
		v.scalars = c
	case StrokePath:
		c.Path.Segments = nil
		c.Brush.GradientStops = nil
		c.Stroke.Dash = nil
		v.scalars = c
	case DrawPolyline:
		c.Points = nil
		c.Brush.GradientStops = nil
		c.Stroke.Dash = nil
		v.scalars = c
	case DrawPoints:
		c.Points = nil
		c.Brush.GradientStops = nil
		v.scalars = c
	case DrawGlyphRun:
		c.Run.Glyphs = nil
		c.Run.GraphemeAdvances = nil
		c.Brush.GradientStops = nil
		v.scalars = c
	case DrawSelectionRects:
		c.Rects = nil
		c.Brush.GradientStops = nil
		v.scalars = c
	case DrawBlurredShadow:
		c.Path.Segments = nil
		v.scalars = c
	}
	return v
}

func (v CommandView) Scalars() Command { return v.scalars }

func (v CommandView) Brush() BrushView {
	switch c := v.payload.(type) {
	case FillRect:
		return ReadBrush(c.Brush)
	case StrokeRect:
		return ReadBrush(c.Brush)
	case FillPath:
		return ReadBrush(c.Brush)
	case StrokePath:
		return ReadBrush(c.Brush)
	case DrawPolyline:
		return ReadBrush(c.Brush)
	case DrawPoints:
		return ReadBrush(c.Brush)
	case DrawGlyphRun:
		return ReadBrush(c.Brush)
	case DrawSelectionRects:
		return ReadBrush(c.Brush)
	}
	return BrushView{}
}

func (v CommandView) Path() PathView {
	switch c := v.payload.(type) {
	case FillPath:
		return ReadPath(c.Path)
	case StrokePath:
		return ReadPath(c.Path)
	case DrawBlurredShadow:
		return ReadPath(c.Path)
	}
	return PathView{}
}

func (v CommandView) Points() Values[Point] {
	switch c := v.payload.(type) {
	case DrawPolyline:
		return Values[Point]{c.Points}
	case DrawPoints:
		return Values[Point]{c.Points}
	}
	return Values[Point]{}
}

func (v CommandView) Rects() Values[Rect] {
	if c, ok := v.payload.(DrawSelectionRects); ok {
		return Values[Rect]{c.Rects}
	}
	return Values[Rect]{}
}

func (v CommandView) StrokeDash() Values[float32] {
	switch c := v.payload.(type) {
	case StrokeRect:
		return Values[float32]{c.Stroke.Dash}
	case StrokePath:
		return Values[float32]{c.Stroke.Dash}
	case DrawPolyline:
		return Values[float32]{c.Stroke.Dash}
	}
	return Values[float32]{}
}

func (v CommandView) Glyphs() Values[text.PositionedGlyph] {
	if c, ok := v.payload.(DrawGlyphRun); ok {
		return Values[text.PositionedGlyph]{c.Run.Glyphs}
	}
	return Values[text.PositionedGlyph]{}
}

func (v CommandView) GraphemeAdvances() Values[float32] {
	if c, ok := v.payload.(DrawGlyphRun); ok {
		return Values[float32]{c.Run.GraphemeAdvances}
	}
	return Values[float32]{}
}

// ExpandStroke uses the owning gfx algorithm without exposing the page's
// mutable source slices. The result belongs to scratch, not the command page.
func (v CommandView) ExpandStroke(scratch *StrokeScratch) []PathSegment {
	switch c := v.payload.(type) {
	case StrokePath:
		return scratch.Expand(c.Path, c.Stroke)
	case StrokeRect:
		scratch.commandPath = appendRectSegments(scratch.commandPath[:0], c.Rect)
		return scratch.Expand(Path{Segments: scratch.commandPath}, c.Stroke)
	case DrawPolyline:
		scratch.commandPath = appendPolylineSegments(scratch.commandPath[:0], c.Points, c.Closed)
		return scratch.Expand(Path{Segments: scratch.commandPath}, c.Stroke)
	default:
		panic("gfx: ExpandStroke requires a stroke command")
	}
}
