package gfx

import (
	"math"
	"testing"
)

func strokePoints(t *testing.T, path Path, stroke StrokeStyle) [][]Point {
	t.Helper()
	segs := ExpandStroke(path, stroke).Segments
	var contours [][]Point
	var cur []Point
	for _, seg := range segs {
		switch seg.Verb {
		case PathMoveTo:
			cur = []Point{seg.Pts[0]}
		case PathLineTo:
			cur = append(cur, seg.Pts[0])
		case PathClose:
			contours = append(contours, cur)
		}
	}
	if len(contours) == 0 {
		t.Fatalf("ExpandStroke produced no contours")
	}
	return contours
}

func ptAlmostEqual(a, b Point) bool {
	return math.Abs(float64(a.X-b.X)) < 1e-3 && math.Abs(float64(a.Y-b.Y)) < 1e-3
}

func hasPoint(pts []Point, want Point) bool {
	for _, p := range pts {
		if ptAlmostEqual(p, want) {
			return true
		}
	}
	return false
}

func contains(t *testing.T, pts []Point, want Point, what string) {
	t.Helper()
	if !hasPoint(pts, want) {
		t.Fatalf("%s: contour lacks point %v (have %v)", what, want, pts)
	}
}

// A single horizontal segment with butt caps must expand to a rectangle.
func TestExpandStroke_SegmentButtIsRect(t *testing.T) {
	path := LinePath(Point{X: 0, Y: 0}, Point{X: 20, Y: 0})
	cs := strokePoints(t, path, DefaultStroke(4))
	if len(cs) != 1 {
		t.Fatalf("want one outline contour, got %d", len(cs))
	}
	c := cs[0]
	contains(t, c, Point{X: 0, Y: -2}, "top-left")
	contains(t, c, Point{X: 20, Y: -2}, "top-right")
	contains(t, c, Point{X: 20, Y: 2}, "bottom-right")
	contains(t, c, Point{X: 0, Y: 2}, "bottom-left")
}

// A segment with round caps must bulge to the left and right by half-width.
func TestExpandStroke_SegmentRoundCaps(t *testing.T) {
	path := LinePath(Point{X: 10, Y: 10}, Point{X: 30, Y: 10})
	stroke := DefaultStroke(4)
	stroke.Cap = LineCapRound
	c := strokePoints(t, path, stroke)[0]
	contains(t, c, Point{X: 8, Y: 10}, "left round cap reaches -h")
	contains(t, c, Point{X: 32, Y: 10}, "right round cap reaches +h")
}

// A segment with square caps must extend h beyond each endpoint.
func TestExpandStroke_SegmentSquareCaps(t *testing.T) {
	path := LinePath(Point{X: 10, Y: 10}, Point{X: 30, Y: 10})
	stroke := DefaultStroke(4)
	stroke.Cap = LineCapSquare
	c := strokePoints(t, path, stroke)[0]
	contains(t, c, Point{X: 8, Y: 8}, "square cap extends left (top corner)")
	contains(t, c, Point{X: 8, Y: 12}, "square cap extends left (bottom corner)")
	contains(t, c, Point{X: 32, Y: 8}, "square cap extends right (top corner)")
}

// A right-angle corner with a miter join must reach the miter point (h*sqrt(2)
// along the corner bisector) on the outer side.
func TestExpandStroke_MiterJoinReachesMiterPoint(t *testing.T) {
	path := NewPath().
		MoveTo(Point{X: 0, Y: 0}).
		LineTo(Point{X: 10, Y: 0}).
		LineTo(Point{X: 10, Y: 10}).
		Build()
	c := strokePoints(t, path, DefaultStroke(4))[0]
	// The outer miter of the right-angle corner sits at (10 - h, h).
	contains(t, c, Point{X: 8, Y: 2}, "outer miter point")
}

// A closed rect stroke must produce the annular ring: two contours with
// opposite winding (checked via signed area), the outer expanded by h and the
// inner contracted by h.
func TestExpandStroke_ClosedRectIsAnnular(t *testing.T) {
	path := RectPath(RectFromXYWH(0, 0, 20, 20))
	cs := strokePoints(t, path, DefaultStroke(4))
	if len(cs) != 2 {
		t.Fatalf("closed stroke must yield two annular contours, got %d", len(cs))
	}
	area := func(pts []Point) float64 {
		var a float64
		for i := 0; i < len(pts); i++ {
			j := (i + 1) % len(pts)
			a += float64(pts[i].X*pts[j].Y - pts[j].X*pts[i].Y)
		}
		return a / 2
	}
	a0, a1 := area(cs[0]), area(cs[1])
	if math.Abs(a0) <= math.Abs(a1) {
		t.Fatalf("outer contour must be the larger one: areas %v, %v", a0, a1)
	}
	if a0*a1 >= 0 {
		t.Fatalf("annular contours must wind oppositely: areas %v, %v", a0, a1)
	}
	// The ring's outer contour reaches the +h offset corners.
	outer := cs[0]
	for _, want := range []Point{{-2, -2}, {22, -2}, {22, 22}, {-2, 22}} {
		contains(t, outer, want, "outer contour corner")
	}
}

// A dashed open line must yield multiple on-pieces, each capped.
func TestExpandStroke_DashYieldsPieces(t *testing.T) {
	path := LinePath(Point{X: 0, Y: 0}, Point{X: 100, Y: 0})
	stroke := DefaultStroke(4)
	stroke.Dash = []float32{10, 5}
	cs := strokePoints(t, path, stroke)
	if len(cs) < 5 {
		t.Fatalf("100px line with 10-on/5-off must produce ~6 pieces, got %d", len(cs))
	}
	for _, c := range cs {
		if len(c) < 4 {
			t.Fatalf("each dash piece must be a closed outline, got %d points", len(c))
		}
	}
}

// The pooled scratch must be allocation-free in steady state (NFR-6).
func TestStrokeScratch_ZeroAllocationsSteadyState(t *testing.T) {
	path := NewPath().
		MoveTo(Point{X: 0, Y: 0}).
		LineTo(Point{X: 10, Y: 0}).
		LineTo(Point{X: 10, Y: 10}).
		Close().
		Build()
	stroke := DefaultStroke(3)
	var s StrokeScratch
	_ = s.Expand(path, stroke) // warm the buffers
	allocs := testing.AllocsPerRun(50, func() {
		_ = s.Expand(path, stroke)
	})
	if allocs != 0 {
		t.Fatalf("stroke expansion must allocate zero bytes steady-state, got %.0f allocs/op", allocs)
	}
}

// Round joins must bulge through the corner bisector (an arc point exists
// between the two offset points on the outer side).
func TestExpandStroke_RoundJoinBulges(t *testing.T) {
	path := NewPath().
		MoveTo(Point{X: 0, Y: 0}).
		LineTo(Point{X: 10, Y: 0}).
		LineTo(Point{X: 10, Y: 10}).
		Build()
	stroke := DefaultStroke(4)
	stroke.Join = LineJoinRound
	c := strokePoints(t, path, stroke)[0]
	// The outer corner arc passes through (10 - h/√2, h/√2) ≈ (8.59, 1.41).
	contains(t, c, Point{X: 8.586, Y: 1.414}, "round join outer arc")
}
