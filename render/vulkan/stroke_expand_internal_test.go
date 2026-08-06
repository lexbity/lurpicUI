//go:build linux && cgo

package vulkan

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
)

// TestRectStrokeAnnulus_MatchesExpandStroke proves the encode path's
// gfx.OffsetContour-based rect annular is byte-identical to the shared
// gfx.ExpandStroke expansion for every join style — the software oracle uses
// ExpandStroke, so the two backends must agree exactly.
func TestRectStrokeAnnulus_MatchesExpandStroke(t *testing.T) {
	rect := gfx.RectFromXYWH(10, 10, 40, 30)
	for _, join := range []gfx.LineJoin{gfx.LineJoinMiter, gfx.LineJoinRound, gfx.LineJoinBevel} {
		stroke := gfx.DefaultStroke(4)
		stroke.Join = join
		stroke.Cap = gfx.LineCapSquare

		var s rectStrokeScratch
		annulus := rectStrokeAnnulus(rect, stroke, &s)
		want := gfx.ExpandStroke(gfx.RectPath(rect), stroke).Segments

		if len(annulus) != len(want) {
			t.Fatalf("join=%v: rect annular %d segments vs ExpandStroke %d", join, len(annulus), len(want))
		}
		for i := range want {
			if annulus[i] != want[i] {
				t.Fatalf("join=%v: segment %d differs: annulus=%v ExpandStroke=%v", join, i, annulus[i], want[i])
			}
		}
	}
}
