package gfx

import "math"

// ExpandStroke flattens `path` and expands it into the closed fill path that
// renders the stroke, honoring the full StrokeStyle: width, caps (butt/round/
// square), joins (miter with limit / round / bevel), and dash patterns.
//
// The output is fully flattened (MoveTo/LineTo/Close only), so the GPU stencil
// fill (Slice 7) and the software oracle rasterize the identical polygon — the
// stroke is expressed once, in gfx, and shared by both backends (the
// render/vulkan/stroke_expand.go encoder integration and the software oracle
// both call it). The output path uses nonzero winding: a closed subpath stroke
// becomes the annular between its two offset contours (opposite winding), an
// open subpath stroke becomes one closed outline with end caps. The offset
// directions follow gfx.OffsetContour's convention (right-hand normal for CW
// contours).
func ExpandStroke(path Path, stroke StrokeStyle) Path {
	var s StrokeScratch
	segs := s.Expand(path, stroke)
	return Path{Segments: append([]PathSegment(nil), segs...)}
}

// Polyline is a flattened subpath: a range [Start, Start+Len) into a shared
// point arena, closed when the path closed it.
type Polyline struct {
	Start, Len int
	Closed     bool
}

// StrokeScratch pools every buffer the expansion writes into so the per-frame
// Vulkan encoder can expand strokes without heap allocations (NFR-6). The
// scratch grows on demand and reuses capacity across frames.
type StrokeScratch struct {
	points       []Point    // shared point arena (flattened subpaths + dash pieces)
	subpaths     []Polyline // flattened subpaths / dash pieces (ranges into points)
	pieces       []Polyline // dash on-pieces (ranges into points)
	out          []PathSegment
	cur          []Point       // current subpath points during flatten
	dashCurrent  []Point       // current dash on-piece
	loopSegs     []PathSegment // closed-loop segment list fed to OffsetContour
	offsetBase   []PathSegment // OffsetContour's base contour for the closed rail
	contourPlus  []Point       // plus rail / open outline accumulator
	contourMinus []Point       // minus rail
}

// Expand appends the stroke's closed fill-path segments to s.out (reused
// across calls) and returns them. The result aliases s.out; the caller must
// consume it before the next Expand.
func (s *StrokeScratch) Expand(path Path, stroke StrokeStyle) []PathSegment {
	s.points = s.points[:0]
	s.subpaths = s.subpaths[:0]
	s.pieces = s.pieces[:0]
	s.out = s.out[:0]
	if len(path.Segments) == 0 || stroke.Width <= 0 {
		return s.out
	}
	h := stroke.Width / 2
	s.flattenPath(path.Segments)

	if len(stroke.Dash) > 0 {
		s.applyDashToAll(stroke.Dash, stroke.DashOffset)
		// The on-pieces become the polylines to stroke (all open, so caps apply).
		s.subpaths = s.pieces
		s.pieces = nil
	}

	for _, poly := range s.subpaths {
		if poly.Len < 2 {
			continue
		}
		if poly.Closed {
			s.appendClosedStroke(poly, h, stroke)
		} else {
			s.appendOpenStroke(poly, h, stroke)
		}
	}
	return s.out
}

const strokeFlattenTolerance float32 = 0.25

func (s *StrokeScratch) flattenPath(segs []PathSegment) {
	s.cur = s.cur[:0]
	start := 0
	started := false
	flush := func(closed bool) {
		if started && len(s.cur)-start >= 2 {
			base := len(s.points)
			s.points = append(s.points, s.cur[start:]...)
			s.subpaths = append(s.subpaths, Polyline{Start: base, Len: len(s.cur) - start, Closed: closed})
		}
		started = false
	}
	for _, seg := range segs {
		switch seg.Verb {
		case PathMoveTo:
			flush(false)
			s.cur = append(s.cur, seg.Pts[0])
			start = len(s.cur) - 1
			started = true
		case PathLineTo:
			s.cur = append(s.cur, seg.Pts[0])
		case PathQuadTo:
			p0 := s.cur[len(s.cur)-1]
			s.cur = flattenQuadInto(s.cur, p0, seg.Pts[0], seg.Pts[1], strokeFlattenTolerance, 0)
		case PathCubicTo:
			p0 := s.cur[len(s.cur)-1]
			s.cur = flattenCubicInto(s.cur, p0, seg.Pts[0], seg.Pts[1], seg.Pts[2], strokeFlattenTolerance, 0)
		case PathClose:
			flush(true)
		}
	}
	flush(false)
}

func flattenQuadInto(out []Point, p0, ctrl, p1 Point, tol float32, depth int) []Point {
	if depth >= 32 {
		return append(out, p1)
	}
	mid := Point{X: (p0.X + p1.X) * 0.5, Y: (p0.Y + p1.Y) * 0.5}
	dx := ctrl.X - mid.X
	dy := ctrl.Y - mid.Y
	if dx*dx+dy*dy <= tol*tol {
		return append(out, p1)
	}
	a := Point{X: (p0.X + ctrl.X) * 0.5, Y: (p0.Y + ctrl.Y) * 0.5}
	b := Point{X: (ctrl.X + p1.X) * 0.5, Y: (ctrl.Y + p1.Y) * 0.5}
	m := Point{X: (a.X + b.X) * 0.5, Y: (a.Y + b.Y) * 0.5}
	out = flattenQuadInto(out, p0, a, m, tol, depth+1)
	return flattenQuadInto(out, m, b, p1, tol, depth+1)
}

func flattenCubicInto(out []Point, p0, c1, c2, p1 Point, tol float32, depth int) []Point {
	if depth >= 32 {
		return append(out, p1)
	}
	if pointLineDist(c1, p0, p1) <= tol && pointLineDist(c2, p0, p1) <= tol {
		return append(out, p1)
	}
	a1 := midPoint(p0, c1)
	a2 := midPoint(c1, c2)
	a3 := midPoint(c2, p1)
	b1 := midPoint(a1, a2)
	b2 := midPoint(a2, a3)
	m := midPoint(b1, b2)
	out = flattenCubicInto(out, p0, a1, b1, m, tol, depth+1)
	return flattenCubicInto(out, m, b2, a3, p1, tol, depth+1)
}

func midPoint(a, b Point) Point {
	return Point{X: (a.X + b.X) * 0.5, Y: (a.Y + b.Y) * 0.5}
}

func pointLineDist(p, a, b Point) float32 {
	abx := b.X - a.X
	aby := b.Y - a.Y
	len2 := abx*abx + aby*aby
	if len2 <= 1e-12 {
		dx := p.X - a.X
		dy := p.Y - a.Y
		return float32(math.Sqrt(float64(dx*dx + dy*dy)))
	}
	t := ((p.X-a.X)*abx + (p.Y-a.Y)*aby) / len2
	px := a.X + t*abx
	py := a.Y + t*aby
	dx := p.X - px
	dy := p.Y - py
	return float32(math.Sqrt(float64(dx*dx + dy*dy)))
}

// offsetAndDir returns the offset direction (right-hand normal, as
// gfx.OffsetContour uses) and the unit edge direction for the edge a→b.
func offsetAndDir(a, b Point) (Point, Point) {
	dx := b.X - a.X
	dy := b.Y - a.Y
	l2 := dx*dx + dy*dy
	if l2 <= 1e-12 {
		return Point{}, Point{}
	}
	l := float32(math.Sqrt(float64(l2)))
	return Point{X: dy / l, Y: -dx / l}, Point{X: dx / l, Y: dy / l}
}

func (s *StrokeScratch) applyDashToAll(dash []float32, dashOffset float32) {
	for _, poly := range s.subpaths {
		s.dashPolyline(poly, dash, dashOffset)
	}
}

func (s *StrokeScratch) dashPolyline(poly Polyline, dash []float32, dashOffset float32) {
	pts := s.points[poly.Start : poly.Start+poly.Len]
	if len(pts) < 2 {
		return
	}
	patternLen := float32(0)
	for _, d := range dash {
		patternLen += d
	}
	if patternLen <= 0 {
		s.emitPiece(poly)
		return
	}

	// Phase into the pattern.
	phase := float32(math.Mod(float64(dashOffset), float64(patternLen)))
	if phase < 0 {
		phase += patternLen
	}
	idx := 0
	for idx < len(dash) && phase >= dash[idx] {
		phase -= dash[idx]
		idx = (idx + 1) % len(dash)
	}
	remaining := dash[idx] - phase
	on := idx%2 == 0

	s.dashCurrent = s.dashCurrent[:0]
	emit := func() {
		if len(s.dashCurrent) >= 2 {
			start := len(s.points)
			s.points = append(s.points, s.dashCurrent...)
			s.pieces = append(s.pieces, Polyline{Start: start, Len: len(s.dashCurrent), Closed: false})
		}
		s.dashCurrent = s.dashCurrent[:0]
	}

	nEdges := len(pts) - 1
	if poly.Closed {
		nEdges++
	}
	for e := 0; e < nEdges; e++ {
		a := pts[e%len(pts)]
		b := pts[(e+1)%len(pts)]
		dx := b.X - a.X
		dy := b.Y - a.Y
		edgeLen := float32(math.Sqrt(float64(dx*dx + dy*dy)))
		if edgeLen <= 1e-9 {
			continue
		}
		ux, uy := dx/edgeLen, dy/edgeLen
		posX, posY := a.X, a.Y
		seg := edgeLen
		for seg > 1e-9 {
			step := float32(math.Min(float64(seg), float64(remaining)))
			atX := posX + ux*step
			atY := posY + uy*step
			if on {
				if len(s.dashCurrent) == 0 {
					s.dashCurrent = append(s.dashCurrent, Point{X: posX, Y: posY})
				}
				s.dashCurrent = append(s.dashCurrent, Point{X: atX, Y: atY})
			} else {
				emit()
			}
			posX, posY = atX, atY
			seg -= step
			remaining -= step
			if remaining <= 1e-4 && seg > 1e-9 {
				idx = (idx + 1) % len(dash)
				remaining = dash[idx]
				on = idx%2 == 0
			}
		}
	}
	if on {
		emit()
	} else {
		emit()
	}
}

func (s *StrokeScratch) emitPiece(poly Polyline) {
	start := len(s.points)
	s.points = append(s.points, s.points[poly.Start:poly.Start+poly.Len]...)
	s.pieces = append(s.pieces, Polyline{Start: start, Len: poly.Len, Closed: false})
}

// appendClosedStroke emits the annular stroke of a closed polyline: the two
// offset contours (opposite winding) as two closed subpaths.
func (s *StrokeScratch) appendClosedStroke(poly Polyline, h float32, stroke StrokeStyle) {
	pts := s.points[poly.Start : poly.Start+poly.Len]
	s.contourPlus = s.buildClosedRail(s.contourPlus[:0], pts, +1, h, stroke)
	s.contourMinus = s.buildClosedRail(s.contourMinus[:0], pts, -1, h, stroke)
	s.emitContour(s.contourPlus)
	s.emitContourReversed(s.contourMinus)
}

// appendOpenStroke emits the single closed outline of an open polyline:
// start cap, plus rail with joins, end cap, minus rail reversed with joins.
func (s *StrokeScratch) appendOpenStroke(poly Polyline, h float32, stroke StrokeStyle) {
	pts := s.points[poly.Start : poly.Start+poly.Len]
	n := len(pts)

	outline := s.contourPlus[:0]
	// Plus rail forward.
	n0, _ := offsetAndDir(pts[0], pts[1])
	outline = append(outline, addScaled(pts[0], n0, h))
	for i := 1; i < n-1; i++ {
		nIn, dIn := offsetAndDir(pts[i-1], pts[i])
		nOut, dOut := offsetAndDir(pts[i], pts[i+1])
		a := addScaled(pts[i], nIn, h)
		outline = append(outline, a)
		b := addScaled(pts[i], nOut, h)
		outline = appendJoinTail(outline, a, b, pts[i], dIn, dOut, +1, h, stroke)
	}
	nLast, dLast := offsetAndDir(pts[n-2], pts[n-1])
	endL := addScaled(pts[n-1], nLast, h)
	endR := addScaled(pts[n-1], nLast, -h)
	outline = append(outline, endL)
	outline = appendCapTail(outline, endL, endR, pts[n-1], dLast, h, stroke.Cap, false)

	// Minus rail reversed.
	mr := s.contourMinus[:0]
	n0, _ = offsetAndDir(pts[0], pts[1])
	mr = append(mr, addScaled(pts[0], n0, -h))
	for i := 1; i < n-1; i++ {
		nIn, dIn := offsetAndDir(pts[i-1], pts[i])
		nOut, dOut := offsetAndDir(pts[i], pts[i+1])
		a := addScaled(pts[i], nIn, -h)
		mr = append(mr, a)
		b := addScaled(pts[i], nOut, -h)
		mr = appendJoinTail(mr, a, b, pts[i], dIn, dOut, -1, h, stroke)
	}
	s.contourMinus = mr
	for i := len(mr) - 1; i >= 0; i-- {
		outline = append(outline, mr[i])
	}

	// Start cap (at pts[0], bulging backward).
	startL := addScaled(pts[0], n0, h)
	startR := addScaled(pts[0], n0, -h)
	_, d0 := offsetAndDir(pts[0], pts[1])
	outline = appendCapTail(outline, startR, startL, pts[0], d0, h, stroke.Cap, true)

	s.contourPlus = outline
	s.emitContour(outline)
}

// buildClosedRail appends the offset rail of a closed polyline for the given
// side (+1 = right-hand normal, -1 = opposite), applying the join geometry at
// every vertex. The rail is closed (its last point equals its first).
//
// The base offset points come from gfx.OffsetContour (the offset of each
// edge's destination along the right-hand normal); the joins reshape each
// corner between consecutive base points.
func (s *StrokeScratch) buildClosedRail(dst []Point, pts []Point, side float32, h float32, stroke StrokeStyle) []Point {
	n := len(pts)
	if n < 3 {
		return dst
	}
	// The loop's segment list for OffsetContour: the explicit closing edge so
	// base[n] carries pts[0]'s offset along the last edge (A_0).
	loopSegs := s.loopSegs[:0]
	loopSegs = append(loopSegs, PathSegment{Verb: PathMoveTo, Pts: [3]Point{pts[0]}})
	for i := 1; i < n; i++ {
		loopSegs = append(loopSegs, PathSegment{Verb: PathLineTo, Pts: [3]Point{pts[i]}})
	}
	loopSegs = append(loopSegs, PathSegment{Verb: PathLineTo, Pts: [3]Point{pts[0]}})
	loopSegs = append(loopSegs, PathSegment{Verb: PathClose})
	s.loopSegs = loopSegs

	// gfx.OffsetContour computes the per-destination offsets: base[i] for
	// i in 1..n is the offset of pts[i] perpendicular to edge i-1 (with
	// pts[n] = pts[0]); base[0] is the MoveTo's centroid-radial offset, which
	// the rail does not use.
	s.offsetBase = OffsetContourInto(s.offsetBase[:0], loopSegs, side*h)
	return RejoinOffsetContour(dst, s.offsetBase, pts, side, h, stroke)
}

// RejoinOffsetContour refines OffsetContour's base contour into the joined
// offset rail of a closed polyline: at every vertex the incoming offset (base)
// is connected to the outgoing offset (derived from the next base point minus
// the edge) with the StrokeStyle's join geometry. The rail is closed. This is
// shared by the closed-loop stroke expansion (gfx.ExpandStroke) and the Vulkan
// encoder's rect-stroke path, so both produce byte-identical geometry.
func RejoinOffsetContour(dst []Point, base []PathSegment, pts []Point, side float32, h float32, stroke StrokeStyle) []Point {
	n := len(pts)
	if n < 3 || len(base) < n+1 {
		return dst
	}
	a0 := base[n].Pts[0] // the closing edge's destination offset = A_0
	dst = append(dst, a0)
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		// B_i = pts[i] + side*h*n[i] = base[i+1] - (pts[j] - pts[i]).
		b := Point{
			X: base[i+1].Pts[0].X - (pts[j].X - pts[i].X),
			Y: base[i+1].Pts[0].Y - (pts[j].Y - pts[i].Y),
		}
		_, dIn := offsetAndDir(pts[(i+n-1)%n], pts[i])
		_, dOut := offsetAndDir(pts[i], pts[j])
		dst = appendJoinTail(dst, dst[len(dst)-1], b, pts[i], dIn, dOut, side, h, stroke)
	}
	return append(dst, a0)
}

// appendJoinTail appends the join geometry connecting the already-emitted
// point `a` to `b` around the vertex `p` (both a and b at distance h from p).
// For a miter join the offset lines' intersection is used when it lies on the
// offset side and within the miter limit; otherwise (and for bevel) a straight
// segment closes the corner; for round, an arc of radius h bulges through the
// corner's outward bisector.
func appendJoinTail(dst []Point, a, b, p, dIn, dOut Point, side float32, h float32, stroke StrokeStyle) []Point {
	switch stroke.Join {
	case LineJoinRound:
		uax := a.X - p.X
		uay := a.Y - p.Y
		ubx := b.X - p.X
		uby := b.Y - p.Y
		mx := uax + ubx
		my := uay + uby
		ml := float32(math.Sqrt(float64(mx*mx + my*my)))
		if ml <= 1e-6 {
			return append(dst, b)
		}
		return appendArcTail(dst, a, b, p, Point{X: mx / ml, Y: my / ml}, h)
	case LineJoinMiter:
		if m, ok := miterPoint(a, b, dIn, dOut); ok {
			dx := m.X - p.X
			dy := m.Y - p.Y
			if side*cross(dIn.X, dIn.Y, dx, dy) < 0 &&
				side*cross(dOut.X, dOut.Y, dx, dy) < 0 &&
				dx*dx+dy*dy <= stroke.MiterLimit*stroke.MiterLimit*h*h {
				return append(dst, m)
			}
		}
		return append(dst, b)
	default: // LineJoinBevel
		return append(dst, b)
	}
}

// appendCapTail appends the end-cap points connecting the two side offsets
// `from` and `to` (opposite points at distance h around `p`), along the edge
// direction `d`. For a square cap the boundary extends h beyond the endpoint;
// for round it follows the disk arc; for butt it closes straight.
func appendCapTail(dst []Point, from, to, p, d Point, h float32, cap LineCap, start bool) []Point {
	switch cap {
	case LineCapSquare:
		ex, ey := d.X*h, d.Y*h
		if start {
			ex, ey = -ex, -ey
		}
		return append(dst, Point{X: from.X + ex, Y: from.Y + ey}, Point{X: to.X + ex, Y: to.Y + ey}, to)
	case LineCapRound:
		ex, ey := d.X, d.Y
		if start {
			ex, ey = -ex, -ey
		}
		return appendArcTail(dst, from, to, p, Point{X: ex, Y: ey}, h)
	default: // LineCapButt
		return append(dst, to)
	}
}

// appendArcTail appends the arc points from `from` to `to` around `center`
// (radius r), bulging through `midDir`, ending with `to`.
func appendArcTail(dst []Point, from, to, center, midDir Point, r float32) []Point {
	angFrom := math.Atan2(float64(from.Y-center.Y), float64(from.X-center.X))
	angTo := math.Atan2(float64(to.Y-center.Y), float64(to.X-center.X))
	angMid := math.Atan2(float64(midDir.Y), float64(midDir.X))
	sweep1 := wrapAngle(angMid - angFrom)
	sweep2 := wrapAngle(angTo - angMid)
	total := sweep1 + sweep2
	n := arcSegments(total, r)
	for k := 1; k <= n; k++ {
		tt := float64(k) / float64(n)
		a := angFrom + total*tt
		dst = append(dst, Point{
			X: center.X + float32(math.Cos(a))*r,
			Y: center.Y + float32(math.Sin(a))*r,
		})
	}
	return dst
}

func wrapAngle(a float64) float64 {
	for a > math.Pi {
		a -= 2 * math.Pi
	}
	for a < -math.Pi {
		a += 2 * math.Pi
	}
	return a
}

func arcSegments(radians float64, r float32) int {
	const maxChord = 0.25
	if r <= 0 {
		return 1
	}
	step := 2 * math.Acos(1-float64(maxChord)/float64(r))
	if step <= 0 || step > math.Pi {
		return 16
	}
	n := int(math.Ceil(math.Abs(radians) / step))
	if n < 1 {
		n = 1
	}
	if n > 128 {
		n = 128
	}
	return n
}

// miterPoint returns the intersection of the line through `a` (direction dIn)
// and the line through `b` (direction dOut).
func miterPoint(a, b, dIn, dOut Point) (Point, bool) {
	denom := dIn.X*dOut.Y - dIn.Y*dOut.X
	if math.Abs(float64(denom)) < 1e-9 {
		return Point{}, false
	}
	bx := b.X - a.X
	by := b.Y - a.Y
	t := (bx*dOut.Y - by*dOut.X) / denom
	return Point{X: a.X + t*dIn.X, Y: a.Y + t*dIn.Y}, true
}

func cross(ax, ay, bx, by float32) float32 {
	return ax*by - ay*bx
}

func addScaled(p, n Point, k float32) Point {
	return Point{X: p.X + n.X*k, Y: p.Y + n.Y*k}
}

// emitContour appends a closed contour (MoveTo, LineTo..., Close) to s.out.
func (s *StrokeScratch) emitContour(pts []Point) {
	if len(pts) < 3 {
		return
	}
	s.out = append(s.out, PathSegment{Verb: PathMoveTo, Pts: [3]Point{pts[0]}})
	for i := 1; i < len(pts); i++ {
		s.out = append(s.out, PathSegment{Verb: PathLineTo, Pts: [3]Point{pts[i]}})
	}
	s.out = append(s.out, PathSegment{Verb: PathClose})
}

// emitContourReversed appends the closed contour in reverse point order (used
// for the inner offset rail so the annular winds oppositely).
func (s *StrokeScratch) emitContourReversed(pts []Point) {
	if len(pts) < 3 {
		return
	}
	s.out = append(s.out, PathSegment{Verb: PathMoveTo, Pts: [3]Point{pts[len(pts)-1]}})
	for i := len(pts) - 2; i >= 0; i-- {
		s.out = append(s.out, PathSegment{Verb: PathLineTo, Pts: [3]Point{pts[i]}})
	}
	s.out = append(s.out, PathSegment{Verb: PathClose})
}
