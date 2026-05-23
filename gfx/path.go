package gfx

import "math"

type PathVerb uint8

const (
	PathMoveTo PathVerb = iota
	PathLineTo
	PathQuadTo
	PathCubicTo
	PathClose
)

type PathSegment struct {
	Verb PathVerb
	Pts  [3]Point
}

type Path struct {
	Segments []PathSegment
}

type PathBuilder struct {
	segments []PathSegment
}

func NewPath() *PathBuilder {
	return &PathBuilder{}
}

func (b *PathBuilder) MoveTo(p Point) *PathBuilder {
	b.segments = append(b.segments, PathSegment{
		Verb: PathMoveTo,
		Pts:  [3]Point{p},
	})
	return b
}

func (b *PathBuilder) LineTo(p Point) *PathBuilder {
	b.segments = append(b.segments, PathSegment{
		Verb: PathLineTo,
		Pts:  [3]Point{p},
	})
	return b
}

func (b *PathBuilder) QuadTo(ctrl, dest Point) *PathBuilder {
	b.segments = append(b.segments, PathSegment{
		Verb: PathQuadTo,
		Pts:  [3]Point{ctrl, dest},
	})
	return b
}

func (b *PathBuilder) CubicTo(c1, c2, dest Point) *PathBuilder {
	b.segments = append(b.segments, PathSegment{
		Verb: PathCubicTo,
		Pts:  [3]Point{c1, c2, dest},
	})
	return b
}

func (b *PathBuilder) Close() *PathBuilder {
	b.segments = append(b.segments, PathSegment{Verb: PathClose})
	return b
}

func (b *PathBuilder) Build() Path {
	if b == nil || len(b.segments) == 0 {
		return Path{}
	}
	segments := make([]PathSegment, len(b.segments))
	copy(segments, b.segments)
	return Path{Segments: segments}
}

func RectPath(r Rect) Path {
	if r.IsEmpty() {
		return Path{}
	}

	return NewPath().
		MoveTo(Point{X: r.Min.X, Y: r.Min.Y}).
		LineTo(Point{X: r.Max.X, Y: r.Min.Y}).
		LineTo(Point{X: r.Max.X, Y: r.Max.Y}).
		LineTo(Point{X: r.Min.X, Y: r.Max.Y}).
		Close().
		Build()
}

func RoundedRectPath(r Rect, radius float32) Path {
	if r.IsEmpty() {
		return Path{}
	}

	maxRadius := float32(math.Min(float64(r.Width()), float64(r.Height()))) / 2
	if radius <= 0 {
		return RectPath(r)
	}
	if radius > maxRadius {
		radius = maxRadius
	}

	minX, minY := r.Min.X, r.Min.Y
	maxX, maxY := r.Max.X, r.Max.Y
	rx := radius
	ry := radius

	return NewPath().
		MoveTo(Point{X: minX + rx, Y: minY}).
		LineTo(Point{X: maxX - rx, Y: minY}).
		QuadTo(Point{X: maxX, Y: minY}, Point{X: maxX, Y: minY + ry}).
		LineTo(Point{X: maxX, Y: maxY - ry}).
		QuadTo(Point{X: maxX, Y: maxY}, Point{X: maxX - rx, Y: maxY}).
		LineTo(Point{X: minX + rx, Y: maxY}).
		QuadTo(Point{X: minX, Y: maxY}, Point{X: minX, Y: maxY - ry}).
		LineTo(Point{X: minX, Y: minY + ry}).
		QuadTo(Point{X: minX, Y: minY}, Point{X: minX + rx, Y: minY}).
		Close().
		Build()
}

func CirclePath(center Point, radius float32) Path {
	if radius <= 0 {
		return Path{}
	}

	k := float32(0.552284749831) * radius
	return NewPath().
		MoveTo(Point{X: center.X + radius, Y: center.Y}).
		CubicTo(
			Point{X: center.X + radius, Y: center.Y + k},
			Point{X: center.X + k, Y: center.Y + radius},
			Point{X: center.X, Y: center.Y + radius},
		).
		CubicTo(
			Point{X: center.X - k, Y: center.Y + radius},
			Point{X: center.X - radius, Y: center.Y + k},
			Point{X: center.X - radius, Y: center.Y},
		).
		CubicTo(
			Point{X: center.X - radius, Y: center.Y - k},
			Point{X: center.X - k, Y: center.Y - radius},
			Point{X: center.X, Y: center.Y - radius},
		).
		CubicTo(
			Point{X: center.X + k, Y: center.Y - radius},
			Point{X: center.X + radius, Y: center.Y - k},
			Point{X: center.X + radius, Y: center.Y},
		).
		Close().
		Build()
}

func PolylinePath(pts []Point, closed bool) Path {
	if len(pts) == 0 {
		return Path{}
	}

	builder := NewPath().MoveTo(pts[0])
	for i := 1; i < len(pts); i++ {
		builder.LineTo(pts[i])
	}
	if closed {
		builder.Close()
	}
	return builder.Build()
}

func LinePath(start, end Point) Path {
	return NewPath().
		MoveTo(start).
		LineTo(end).
		Build()
}

