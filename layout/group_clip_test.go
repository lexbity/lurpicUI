package layout

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

func TestGroupClipsContent_respectsVisibleContract(t *testing.T) {
	parent := facet.GroupParentContract{
		Clipping: facet.GroupClipVisible,
		Overflow: facet.OverflowClip,
	}
	if GroupClipsContent(parent) {
		t.Fatal("expected visible clip contract to bypass clipping")
	}
}

func TestIntersectClipRects_returnsOverlap(t *testing.T) {
	got, ok := IntersectClipRects(
		gfx.RectFromXYWH(0, 0, 100, 100),
		true,
		gfx.RectFromXYWH(50, 50, 100, 100),
	)
	if !ok {
		t.Fatal("expected overlap")
	}
	want := gfx.RectFromXYWH(50, 50, 50, 50)
	if got != want {
		t.Fatalf("intersection = %#v, want %#v", got, want)
	}
}
