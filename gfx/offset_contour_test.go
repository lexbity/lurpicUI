package gfx

import (
	"math"
	"testing"
)

func TestOffsetContour_zero_d_is_noop(t *testing.T) {
	segs := RoundedRectPath(RectFromXYWH(0, 0, 100, 50), 5).Segments
	got := OffsetContour(segs, 0)
	if len(got) != len(segs) {
		t.Fatalf("len = %d, want %d", len(got), len(segs))
	}
	for i := range segs {
		if segs[i].Verb != got[i].Verb {
			t.Fatalf("segment %d verb changed", i)
		}
	}
}

func TestOffsetContour_nil_input(t *testing.T) {
	got := OffsetContour(nil, 2)
	if len(got) != 0 {
		t.Fatal("expected empty")
	}
}

func TestOffsetContour_empty_input(t *testing.T) {
	got := OffsetContour([]PathSegment{}, 2)
	if len(got) != 0 {
		t.Fatal("expected empty")
	}
}

func TestOffsetContour_rect_straight_edges_remain_parallel(t *testing.T) {
	// A 100x50 rect with 5px radius.
	rect := RectFromXYWH(10, 10, 100, 50)
	segs := RoundedRectPath(rect, 5).Segments

	// Offset outward by 2px.
	out := OffsetContour(segs, 2)

	// The top edge of the outer contour should be at Y=8 (10-2).
	// The top edge is a LineTo segment.
	for i, seg := range out {
		if seg.Verb == PathLineTo {
			// Find the corresponding original segment to identify which edge.
			origSeg := segs[i]
			if origSeg.Verb != PathLineTo {
				continue
			}
			origDest := origSeg.Pts[0]
			newDest := seg.Pts[0]

			// Top edge: Y should decrease by 2 (move up).
			if origDest.Y == rect.Min.Y {
				if newDest.Y != rect.Min.Y-2 {
					t.Errorf("top edge endpoint %d: Y=%f, want %f", i, newDest.Y, rect.Min.Y-2)
				}
				// X should stay on the same vertical line (no horizontal drift).
				if newDest.X != origDest.X {
					t.Errorf("top edge endpoint %d: X=%f, want %f (no horizontal drift)", i, newDest.X, origDest.X)
				}
			}
			// Right edge: X should increase by 2.
			if origDest.X == rect.Max.X {
				if newDest.X != rect.Max.X+2 {
					t.Errorf("right edge endpoint %d: X=%f, want %f", i, newDest.X, rect.Max.X+2)
				}
				if newDest.Y != origDest.Y {
					t.Errorf("right edge endpoint %d: Y drifted from %f to %f", i, origDest.Y, newDest.Y)
				}
			}
			// Bottom edge: Y should increase by 2.
			if origDest.Y == rect.Max.Y {
				if newDest.Y != rect.Max.Y+2 {
					t.Errorf("bottom edge endpoint %d: Y=%f, want %f", i, newDest.Y, rect.Max.Y+2)
				}
				if newDest.X != origDest.X {
					t.Errorf("bottom edge endpoint %d: X drifted from %f to %f", i, origDest.X, newDest.X)
				}
			}
			// Left edge: X should decrease by 2.
			if origDest.X == rect.Min.X {
				if newDest.X != rect.Min.X-2 {
					t.Errorf("left edge endpoint %d: X=%f, want %f", i, newDest.X, rect.Min.X-2)
				}
				if newDest.Y != origDest.Y {
					t.Errorf("left edge endpoint %d: Y drifted from %f to %f", i, origDest.Y, newDest.Y)
				}
			}
		}
	}
}

func TestOffsetContour_rect_inner_contour_moves_inward(t *testing.T) {
	rect := RectFromXYWH(10, 10, 100, 50)
	segs := RoundedRectPath(rect, 5).Segments

	// Contract inward by 2px.
	inner := OffsetContour(segs, -2)

	for i, seg := range inner {
		if seg.Verb == PathLineTo {
			origSeg := segs[i]
			if origSeg.Verb != PathLineTo {
				continue
			}
			origDest := origSeg.Pts[0]
			newDest := seg.Pts[0]

			// Top edge: Y should increase by 2 (move down into interior).
			if origDest.Y == rect.Min.Y {
				if newDest.Y != rect.Min.Y+2 {
					t.Errorf("inner top edge endpoint %d: Y=%f, want %f", i, newDest.Y, rect.Min.Y+2)
				}
				if newDest.X != origDest.X {
					t.Errorf("inner top edge endpoint %d: X=%f, want %f (no horizontal drift)", i, newDest.X, origDest.X)
				}
			}
			// Right edge: X should decrease by 2 (move left into interior).
			if origDest.X == rect.Max.X {
				if newDest.X != rect.Max.X-2 {
					t.Errorf("inner right edge endpoint %d: X=%f, want %f", i, newDest.X, rect.Max.X-2)
				}
			}
			// Bottom edge: Y should decrease by 2 (move up into interior).
			if origDest.Y == rect.Max.Y {
				if newDest.Y != rect.Max.Y-2 {
					t.Errorf("inner bottom edge endpoint %d: Y=%f, want %f", i, newDest.Y, rect.Max.Y-2)
				}
			}
			// Left edge: X should increase by 2 (move right into interior).
			if origDest.X == rect.Min.X {
				if newDest.X != rect.Min.X+2 {
					t.Errorf("inner left edge endpoint %d: X=%f, want %f", i, newDest.X, rect.Min.X+2)
				}
			}
		}
	}
}

// TestOffsetContour_stroke_band_width verifies that the distance between the
// outer and inner offset contours along straight edges equals 2*d (the full
// stroke width).
func TestOffsetContour_stroke_band_width(t *testing.T) {
	rect := RectFromXYWH(20, 20, 60, 40)
	segs := RoundedRectPath(rect, 4).Segments
	half := float32(3)

	outer := OffsetContour(segs, half)
	inner := OffsetContour(segs, -half)

	for i := range outer {
		if outer[i].Verb != PathLineTo || inner[i].Verb != PathLineTo {
			continue
		}

		op := outer[i].Pts[0]
		ip := inner[i].Pts[0]
		dx := op.X - ip.X
		dy := op.Y - ip.Y
		dist := float32(math.Sqrt(float64(dx*dx + dy*dy)))

		// Expect the band width to be exactly 2*half = 6 along straight edges.
		if dist < 5.9 || dist > 6.1 {
			t.Errorf("segment %d: band width = %f, want ~6", i, dist)
		}
	}
}
