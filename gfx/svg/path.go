package svg

import (
	"math"

	. "codeburg.org/lexbit/lurpicui/gfx"
)

// Bounds returns the axis-aligned bounds of the path.
func Bounds(path Path) Rect {
	return pathBounds(path)
}

// Transformed returns a copy of the path with the transform applied to every point.
func Transformed(path Path, t Transform) Path {
	if len(path.Segments) == 0 || t.IsIdentity() {
		segments := make([]PathSegment, len(path.Segments))
		copy(segments, path.Segments)
		return Path{Segments: segments}
	}
	out := make([]PathSegment, len(path.Segments))
	for i, seg := range path.Segments {
		out[i] = seg
		switch seg.Verb {
		case PathMoveTo, PathLineTo:
			out[i].Pts[0] = t.TransformPoint(seg.Pts[0])
		case PathQuadTo:
			out[i].Pts[0] = t.TransformPoint(seg.Pts[0])
			out[i].Pts[1] = t.TransformPoint(seg.Pts[1])
		case PathCubicTo:
			out[i].Pts[0] = t.TransformPoint(seg.Pts[0])
			out[i].Pts[1] = t.TransformPoint(seg.Pts[1])
			out[i].Pts[2] = t.TransformPoint(seg.Pts[2])
		}
	}
	return Path{Segments: out}
}

func pathBounds(path Path) Rect {
	var bounds Rect
	first := true
	var current Point
	var subpathStart Point
	for _, seg := range path.Segments {
		switch seg.Verb {
		case PathMoveTo:
			current = seg.Pts[0]
			subpathStart = current
			bounds = includePoint(bounds, &first, current)
		case PathLineTo:
			current = seg.Pts[0]
			bounds = includePoint(bounds, &first, current)
		case PathQuadTo:
			p0 := current
			p1 := seg.Pts[0]
			p2 := seg.Pts[1]
			bounds = includePoint(bounds, &first, p0)
			bounds = includePoint(bounds, &first, p2)
			for _, t := range quadExtremaTs(p0.X, p1.X, p2.X) {
				bounds = includePoint(bounds, &first, quadPoint(p0, p1, p2, t))
			}
			for _, t := range quadExtremaTs(p0.Y, p1.Y, p2.Y) {
				bounds = includePoint(bounds, &first, quadPoint(p0, p1, p2, t))
			}
			current = p2
		case PathCubicTo:
			p0 := current
			p1 := seg.Pts[0]
			p2 := seg.Pts[1]
			p3 := seg.Pts[2]
			bounds = includePoint(bounds, &first, p0)
			bounds = includePoint(bounds, &first, p3)
			for _, t := range cubicExtremaTs(p0.X, p1.X, p2.X, p3.X) {
				bounds = includePoint(bounds, &first, cubicPoint(p0, p1, p2, p3, t))
			}
			for _, t := range cubicExtremaTs(p0.Y, p1.Y, p2.Y, p3.Y) {
				bounds = includePoint(bounds, &first, cubicPoint(p0, p1, p2, p3, t))
			}
			current = p3
		case PathClose:
			current = subpathStart
		}
	}
	if first {
		return Rect{}
	}
	return bounds
}

func includePoint(bounds Rect, first *bool, p Point) Rect {
	if first != nil && *first {
		*first = false
		return Rect{Min: p, Max: p}
	}
	if p.X < bounds.Min.X {
		bounds.Min.X = p.X
	}
	if p.Y < bounds.Min.Y {
		bounds.Min.Y = p.Y
	}
	if p.X > bounds.Max.X {
		bounds.Max.X = p.X
	}
	if p.Y > bounds.Max.Y {
		bounds.Max.Y = p.Y
	}
	return bounds
}

func quadExtremaTs(p0, p1, p2 float32) []float32 {
	denom := p0 - 2*p1 + p2
	if denom == 0 {
		return nil
	}
	t := (p0 - p1) / denom
	if t > 0 && t < 1 {
		return []float32{t}
	}
	return nil
}

func cubicExtremaTs(p0, p1, p2, p3 float32) []float32 {
	a := -p0 + 3*p1 - 3*p2 + p3
	b := 2 * (p0 - 2*p1 + p2)
	c := p1 - p0
	if a == 0 {
		if b == 0 {
			return nil
		}
		t := -c / b
		if t > 0 && t < 1 {
			return []float32{t}
		}
		return nil
	}
	d := b*b - 4*a*c
	if d < 0 {
		return nil
	}
	sqrtD := float32(math.Sqrt(float64(d)))
	t1 := (-b + sqrtD) / (2 * a)
	t2 := (-b - sqrtD) / (2 * a)
	out := make([]float32, 0, 2)
	if t1 > 0 && t1 < 1 {
		out = append(out, t1)
	}
	if t2 > 0 && t2 < 1 && t2 != t1 {
		out = append(out, t2)
	}
	return out
}

func quadPoint(p0, p1, p2 Point, t float32) Point {
	u := 1 - t
	return Point{
		X: u*u*p0.X + 2*u*t*p1.X + t*t*p2.X,
		Y: u*u*p0.Y + 2*u*t*p1.Y + t*t*p2.Y,
	}
}

func cubicPoint(p0, p1, p2, p3 Point, t float32) Point {
	u := 1 - t
	uu := u * u
	tt := t * t
	return Point{
		X: uu*u*p0.X + 3*uu*t*p1.X + 3*u*tt*p2.X + tt*t*p3.X,
		Y: uu*u*p0.Y + 3*uu*t*p1.Y + 3*u*tt*p2.Y + tt*t*p3.Y,
	}
}

func intersectRects(a, b Rect) Rect {
	if a.IsEmpty() || b.IsEmpty() {
		return Rect{}
	}
	minX := a.Min.X
	if b.Min.X > minX {
		minX = b.Min.X
	}
	minY := a.Min.Y
	if b.Min.Y > minY {
		minY = b.Min.Y
	}
	maxX := a.Max.X
	if b.Max.X < maxX {
		maxX = b.Max.X
	}
	maxY := a.Max.Y
	if b.Max.Y < maxY {
		maxY = b.Max.Y
	}
	if minX >= maxX || minY >= maxY {
		return Rect{}
	}
	return Rect{Min: Point{X: minX, Y: minY}, Max: Point{X: maxX, Y: maxY}}
}
