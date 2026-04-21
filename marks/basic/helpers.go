package basic

import (
	"math"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
)

func min(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func max(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func distance(a, b gfx.Point) float32 {
	dx := float64(a.X - b.X)
	dy := float64(a.Y - b.Y)
	return float32(math.Hypot(dx, dy))
}

func transformAnchors(tx gfx.Transform, anchors layout.AnchorSet) layout.AnchorSet {
	if len(anchors) == 0 {
		return nil
	}
	out := make(layout.AnchorSet, len(anchors))
	for id, pt := range anchors {
		out[id] = tx.TransformPoint(pt)
	}
	return out
}

func pathBounds(path gfx.Path) gfx.Rect {
	if len(path.Segments) == 0 {
		return gfx.Rect{}
	}
	var minPt, maxPt gfx.Point
	havePoint := false
	visit := func(p gfx.Point) {
		if !havePoint {
			minPt = p
			maxPt = p
			havePoint = true
			return
		}
		if p.X < minPt.X {
			minPt.X = p.X
		}
		if p.Y < minPt.Y {
			minPt.Y = p.Y
		}
		if p.X > maxPt.X {
			maxPt.X = p.X
		}
		if p.Y > maxPt.Y {
			maxPt.Y = p.Y
		}
	}
	for _, seg := range path.Segments {
		for _, p := range seg.Pts {
			visit(p)
		}
	}
	if !havePoint {
		return gfx.Rect{}
	}
	return gfx.Rect{Min: minPt, Max: maxPt}
}

func pathAnchorSet(path gfx.Path) layout.AnchorSet {
	bounds := pathBounds(path)
	if bounds.IsEmpty() {
		return nil
	}
	return layout.AnchorSet{
		"bounds-center": {X: bounds.Min.X + bounds.Width()/2, Y: bounds.Min.Y + bounds.Height()/2},
		"top-left":      {X: bounds.Min.X, Y: bounds.Min.Y},
		"top-right":     {X: bounds.Max.X, Y: bounds.Min.Y},
		"bottom-right":  {X: bounds.Max.X, Y: bounds.Max.Y},
		"bottom-left":   {X: bounds.Min.X, Y: bounds.Max.Y},
	}
}

func pointInPolygon(p gfx.Point, pts []gfx.Point, evenOdd bool) bool {
	if len(pts) < 3 {
		return false
	}
	inside := false
	winding := 0
	for i, j := 0, len(pts)-1; i < len(pts); j, i = i, i+1 {
		pi := pts[i]
		pj := pts[j]
		intersects := ((pi.Y > p.Y) != (pj.Y > p.Y)) &&
			(p.X < (pj.X-pi.X)*(p.Y-pi.Y)/(pj.Y-pi.Y+1e-12)+pi.X)
		if evenOdd && intersects {
			inside = !inside
		}
		if !evenOdd {
			if pi.Y <= p.Y {
				if pj.Y > p.Y && cross(pj, pi, p) > 0 {
					winding++
				}
			} else if pj.Y <= p.Y && cross(pj, pi, p) < 0 {
				winding--
			}
		}
	}
	if evenOdd {
		return inside
	}
	return winding != 0
}

func cross(a, b, c gfx.Point) float32 {
	return (a.X-b.X)*(c.Y-b.Y) - (a.Y-b.Y)*(c.X-b.X)
}

func flattenPath(path gfx.Path) [][]gfx.Point {
	if len(path.Segments) == 0 {
		return nil
	}
	var contours [][]gfx.Point
	var pts []gfx.Point
	var current gfx.Point
	var start gfx.Point
	haveStart := false
	flush := func() {
		if len(pts) > 0 {
			contours = append(contours, append([]gfx.Point(nil), pts...))
			pts = pts[:0]
		}
	}
	for _, seg := range path.Segments {
		switch seg.Verb {
		case gfx.PathMoveTo:
			flush()
			current = seg.Pts[0]
			start = current
			haveStart = true
			pts = append(pts, current)
		case gfx.PathLineTo:
			current = seg.Pts[0]
			pts = append(pts, current)
		case gfx.PathQuadTo:
			if !haveStart {
				continue
			}
			ctrl := seg.Pts[0]
			dest := seg.Pts[1]
			const steps = 8
			for i := 1; i <= steps; i++ {
				t := float32(i) / steps
				omt := 1 - t
				p := gfx.Point{
					X: omt*omt*current.X + 2*omt*t*ctrl.X + t*t*dest.X,
					Y: omt*omt*current.Y + 2*omt*t*ctrl.Y + t*t*dest.Y,
				}
				pts = append(pts, p)
			}
			current = dest
		case gfx.PathCubicTo:
			if !haveStart {
				continue
			}
			c1 := seg.Pts[0]
			c2 := seg.Pts[1]
			dest := seg.Pts[2]
			const steps = 12
			for i := 1; i <= steps; i++ {
				t := float32(i) / steps
				omt := 1 - t
				p := gfx.Point{
					X: omt*omt*omt*current.X + 3*omt*omt*t*c1.X + 3*omt*t*t*c2.X + t*t*t*dest.X,
					Y: omt*omt*omt*current.Y + 3*omt*omt*t*c1.Y + 3*omt*t*t*c2.Y + t*t*t*dest.Y,
				}
				pts = append(pts, p)
			}
			current = dest
		case gfx.PathClose:
			if haveStart {
				pts = append(pts, start)
				flush()
				current = start
				haveStart = false
			}
		}
	}
	flush()
	return contours
}

func pathContains(path gfx.Path, p gfx.Point, evenOdd bool) bool {
	contours := flattenPath(path)
	if len(contours) == 0 {
		return false
	}
	if evenOdd {
		inside := false
		for _, contour := range contours {
			if pointInPolygon(p, contour, true) {
				inside = !inside
			}
		}
		return inside
	}
	winding := 0
	for _, contour := range contours {
		if pointInPolygon(p, contour, false) {
			winding++
		}
	}
	return winding != 0
}

func segmentDistance(p, a, b gfx.Point) float32 {
	ax := float64(a.X)
	ay := float64(a.Y)
	bx := float64(b.X)
	by := float64(b.Y)
	px := float64(p.X)
	py := float64(p.Y)

	dx := bx - ax
	dy := by - ay
	if dx == 0 && dy == 0 {
		return float32(math.Hypot(px-ax, py-ay))
	}
	t := ((px-ax)*dx + (py-ay)*dy) / (dx*dx + dy*dy)
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	x := ax + t*dx
	y := ay + t*dy
	return float32(math.Hypot(px-x, py-y))
}

func pathStrokeHit(path gfx.Path, p gfx.Point, width float32) bool {
	if width <= 0 {
		return false
	}
	contours := flattenPath(path)
	if len(contours) == 0 {
		return false
	}
	tolerance := width / 2
	for _, contour := range contours {
		for i := 1; i < len(contour); i++ {
			if segmentDistance(p, contour[i-1], contour[i]) <= tolerance {
				return true
			}
		}
	}
	return false
}
