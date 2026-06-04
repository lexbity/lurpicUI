package gfx

import "math"

// SegmentPointCount returns the number of control/destination points for
// the given verb. MoveTo and LineTo have 1, QuadTo has 2, CubicTo has 3.
func SegmentPointCount(v PathVerb) int {
	switch v {
	case PathMoveTo, PathLineTo:
		return 1
	case PathQuadTo:
		return 2
	case PathCubicTo:
		return 3
	default:
		return 0
	}
}

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

// offsetDir computes the right-hand normal (perpendicular) of the direction
// from a to b, normalized. For a CW closed contour this is the outward direction.
func offsetDir(a, b Point) (Point, float32) {
	dx := b.X - a.X
	dy := b.Y - a.Y
	len2 := dx*dx + dy*dy
	if len2 == 0 {
		return Point{}, 0
	}
	l := float32(math.Sqrt(float64(len2)))
	return Point{X: dy / l, Y: -dx / l}, l
}

// miterDir computes the miter direction at a vertex as the sum of the outward
// normals of the two incident edges. The returned bool is false when the edges
// are degenerate (zero-length).
func miterDir(prev, ctrl, next Point) (Point, bool) {
	n1, l1 := offsetDir(prev, ctrl)
	if l1 == 0 {
		return Point{}, false
	}
	n2, l2 := offsetDir(ctrl, next)
	if l2 == 0 {
		return Point{}, false
	}
	mx := n1.X + n2.X
	my := n1.Y + n2.Y
	ml2 := mx*mx + my*my
	if ml2 == 0 {
		// Opposite normals (180° turn) — fall back to n1
		return n1, true
	}
	ml := float32(math.Sqrt(float64(ml2)))
	return Point{X: mx / ml, Y: my / ml}, true
}

// contourPoint returns the implicit current point after processing all
// segments up to (but not including) index i.
func contourPoint(segs []PathSegment, i int) Point {
	var cur Point
	for j := 0; j < i; j++ {
		switch segs[j].Verb {
		case PathMoveTo, PathLineTo:
			cur = segs[j].Pts[0]
		case PathQuadTo:
			cur = segs[j].Pts[1]
		case PathCubicTo:
			cur = segs[j].Pts[2]
		}
	}
	return cur
}

// OffsetContour returns a new contour offset outward by d (positive d expands,
// negative d contracts). Straight segments are offset perpendicular to the edge
// direction. Curve control points at corners use a miter join. CubicTo segments
// use a centroid-radial approximation as fallback.
//
// The input segments should form a single closed contour with consistent
// winding (CW in screen coordinates). The returned contour preserves the
// segment count and verb structure of the input.
func OffsetContour(segs []PathSegment, d float32) []PathSegment {
	if len(segs) == 0 || d == 0 {
		result := make([]PathSegment, len(segs))
		copy(result, segs)
		return result
	}

	// Compute centroid for fallback on CubicTo and MoveTo.
	var cx, cy float32
	var n int
	for _, seg := range segs {
		c := SegmentPointCount(seg.Verb)
		for j := 0; j < c; j++ {
			cx += seg.Pts[j].X
			cy += seg.Pts[j].Y
			n++
		}
	}
	if n == 0 {
		return nil
	}
	cx /= float32(n)
	cy /= float32(n)

	result := make([]PathSegment, len(segs))
	for i, seg := range segs {
		result[i].Verb = seg.Verb

		switch seg.Verb {
		case PathLineTo:
			prev := contourPoint(segs, i)
			nrm, _ := offsetDir(prev, seg.Pts[0])
			result[i].Pts[0] = Point{
				X: seg.Pts[0].X + nrm.X*d,
				Y: seg.Pts[0].Y + nrm.Y*d,
			}

		case PathQuadTo:
			prev := contourPoint(segs, i)
			ctrl := seg.Pts[0]
			dest := seg.Pts[1]

			if md, ok := miterDir(prev, ctrl, dest); ok {
				result[i].Pts[0] = Point{
					X: ctrl.X + md.X*d,
					Y: ctrl.Y + md.Y*d,
				}
			} else {
				result[i].Pts[0] = ctrl
			}

			nrm, _ := offsetDir(ctrl, dest)
			if nrm.X != 0 || nrm.Y != 0 {
				result[i].Pts[1] = Point{
					X: dest.X + nrm.X*d,
					Y: dest.Y + nrm.Y*d,
				}
			} else {
				result[i].Pts[1] = dest
			}

		default:
			// MoveTo, CubicTo, Close: centroid-radial fallback.
			c := SegmentPointCount(seg.Verb)
			for j := 0; j < c; j++ {
				p := seg.Pts[j]
				dx := p.X - cx
				dy := p.Y - cy
				len2 := dx*dx + dy*dy
				if len2 > 0 {
					l := float32(math.Sqrt(float64(len2)))
					result[i].Pts[j] = Point{X: p.X + dx/l*d, Y: p.Y + dy/l*d}
				} else {
					result[i].Pts[j] = p
				}
			}
		}
	}
	return result
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

