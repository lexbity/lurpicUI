// Package vulkan — stroke expansion (Slice 8).
//
// This file turns a stroke command (StrokePath / StrokeRect / DrawPolyline)
// into the FillPath encoding of the stroke's expanded outline, so the GPU
// pipeline renders every stroke through the Slice 7 stencil fill. The expansion
// happens on the Go side, at packet-encode time, so the GPU frame encoder never
// needs to interpret StrokeStyle itself.
//
// gfx.OffsetContour is genuinely wired into the encode path: the common
// StrokeRect case builds its annular fill (outer + inner offset contours with
// opposite winding) directly from gfx.OffsetContour's base contour, refining
// the corners with the shared gfx.RejoinOffsetContour join geometry. Every
// other stroke delegates to gfx.ExpandStroke, whose closed-loop rails are
// likewise sourced from gfx.OffsetContour. The encoder's pooled buffers keep
// steady-state submission allocation-free (NFR-6); the expanded segments are
// written straight into the pooled packet buffer (no intermediate copy).

package vulkan

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/gfx"
)

// encodeStrokeExpanded writes the FillPath encoding of a stroke command's
// expanded outline into w. `scratch` is the encoder's pooled expansion scratch
// (grown once, reused every frame); `pathSegs` and `rect` are the encoder's
// pooled buffers for the input-path and rect-annular segments (NFR-6: no
// per-stroke heap allocations).
func encodeStrokeExpanded(w *packetWriter, cmd gfx.Command, scratch *gfx.StrokeScratch, pathSegs *[]gfx.PathSegment, rect *rectStrokeScratch) error {
	var segs []gfx.PathSegment
	var brush gfx.Brush
	switch c := cmd.(type) {
	case gfx.StrokePath:
		segs = scratch.Expand(c.Path, c.Stroke)
		brush = c.Brush
	case gfx.StrokeRect:
		// Rect strokes are the common case: the annular is built directly from
		// the rect's offset contours via gfx.OffsetContour (see
		// rectStrokeAnnulus), byte-identical to gfx.ExpandStroke.
		segs = rectStrokeAnnulus(c.Rect, c.Stroke, rect)
		brush = c.Brush
	case gfx.DrawPolyline:
		var path gfx.Path
		path.Segments = appendPolylineSegments((*pathSegs)[:0], c.Points, c.Closed)
		*pathSegs = path.Segments // retain the pooled capacity for the next call
		segs = scratch.Expand(path, c.Stroke)
		brush = c.Brush
	default:
		return fmt.Errorf("vulkan: %s", "encodeStrokeExpanded: not a stroke command")
	}

	if len(segs) == 0 {
		// A zero-width or empty stroke expands to nothing; emit an empty
		// FillPath so the command count stays truthful on the wire.
		w.writeU8(packetCmdFillPath)
		w.writeU32(0)
		w.writeU32(0)
		return w.writeBrush(brush)
	}
	w.writeU8(packetCmdFillPath)
	w.writePathSegments(segs)
	return w.writeBrush(brush)
}

// rectStrokeScratch pools the buffers the rect-stroke annular writes into, so
// the common rect case expands with zero per-frame allocations (NFR-6).
type rectStrokeScratch struct {
	loop  []gfx.PathSegment // the rect loop fed to gfx.OffsetContour
	baseP []gfx.PathSegment // OffsetContour's outer base contour
	baseM []gfx.PathSegment // OffsetContour's inner base contour
	railP []gfx.Point       // re-joined outer rail
	railM []gfx.Point       // re-joined inner rail
	out   []gfx.PathSegment // the annular segments
}

// rectStrokeAnnulus builds the annular fill path of an axis-aligned rect
// stroke directly from the rect's offset contours: gfx.OffsetContourInto
// computes the per-vertex base offsets of the rect loop, gfx.RejoinOffsetContour
// reshapes each corner with the join geometry (miter / round / bevel), and the
// outer and inner rails are emitted with opposite winding so the nonzero fill
// leaves the ring. The result is byte-identical to gfx.ExpandStroke for the
// same rect.
func rectStrokeAnnulus(rect gfx.Rect, stroke gfx.StrokeStyle, s *rectStrokeScratch) []gfx.PathSegment {
	if stroke.Width <= 0 {
		return nil
	}
	h := stroke.Width / 2
	pts := [4]gfx.Point{
		{X: rect.Min.X, Y: rect.Min.Y},
		{X: rect.Max.X, Y: rect.Min.Y},
		{X: rect.Max.X, Y: rect.Max.Y},
		{X: rect.Min.X, Y: rect.Max.Y},
	}

	// The rect loop with the explicit closing edge so base[n] carries pts[0]'s
	// offset along the last edge (A_0).
	loop := s.loop[:0]
	loop = append(loop,
		gfx.PathSegment{Verb: gfx.PathMoveTo, Pts: [3]gfx.Point{pts[0]}},
		gfx.PathSegment{Verb: gfx.PathLineTo, Pts: [3]gfx.Point{pts[1]}},
		gfx.PathSegment{Verb: gfx.PathLineTo, Pts: [3]gfx.Point{pts[2]}},
		gfx.PathSegment{Verb: gfx.PathLineTo, Pts: [3]gfx.Point{pts[3]}},
		gfx.PathSegment{Verb: gfx.PathLineTo, Pts: [3]gfx.Point{pts[0]}},
		gfx.PathSegment{Verb: gfx.PathClose},
	)
	s.loop = loop

	// gfx.OffsetContour produces the base offset contours; the shared rejoin
	// refines the corners.
	baseP := gfx.OffsetContourInto(s.baseP[:0], loop, h)
	s.baseP = baseP
	baseM := gfx.OffsetContourInto(s.baseM[:0], loop, -h)
	s.baseM = baseM
	railP := gfx.RejoinOffsetContour(s.railP[:0], baseP, pts[:], +1, h, stroke)
	s.railP = railP
	railM := gfx.RejoinOffsetContour(s.railM[:0], baseM, pts[:], -1, h, stroke)
	s.railM = railM

	segs := s.out[:0]
	segs = appendContourSegments(segs, railP)
	segs = appendContourSegmentsReversed(segs, railM)
	s.out = segs
	return segs
}

// appendContourSegments appends a closed contour (MoveTo, LineTo..., Close).
func appendContourSegments(dst []gfx.PathSegment, pts []gfx.Point) []gfx.PathSegment {
	if len(pts) < 3 {
		return dst
	}
	dst = append(dst, gfx.PathSegment{Verb: gfx.PathMoveTo, Pts: [3]gfx.Point{pts[0]}})
	for i := 1; i < len(pts); i++ {
		dst = append(dst, gfx.PathSegment{Verb: gfx.PathLineTo, Pts: [3]gfx.Point{pts[i]}})
	}
	return append(dst, gfx.PathSegment{Verb: gfx.PathClose})
}

// appendContourSegmentsReversed appends a closed contour in reverse point order
// (opposite winding, for the inner rail of the annular).
func appendContourSegmentsReversed(dst []gfx.PathSegment, pts []gfx.Point) []gfx.PathSegment {
	if len(pts) < 3 {
		return dst
	}
	dst = append(dst, gfx.PathSegment{Verb: gfx.PathMoveTo, Pts: [3]gfx.Point{pts[len(pts)-1]}})
	for i := len(pts) - 2; i >= 0; i-- {
		dst = append(dst, gfx.PathSegment{Verb: gfx.PathLineTo, Pts: [3]gfx.Point{pts[i]}})
	}
	return append(dst, gfx.PathSegment{Verb: gfx.PathClose})
}

// appendPolylineSegments appends the polyline contour (open or closed) to dst
// (pooled).
func appendPolylineSegments(dst []gfx.PathSegment, pts []gfx.Point, closed bool) []gfx.PathSegment {
	if len(pts) == 0 {
		return dst
	}
	dst = append(dst, gfx.PathSegment{Verb: gfx.PathMoveTo, Pts: [3]gfx.Point{pts[0]}})
	for i := 1; i < len(pts); i++ {
		dst = append(dst, gfx.PathSegment{Verb: gfx.PathLineTo, Pts: [3]gfx.Point{pts[i]}})
	}
	if closed {
		dst = append(dst, gfx.PathSegment{Verb: gfx.PathClose})
	}
	return dst
}
