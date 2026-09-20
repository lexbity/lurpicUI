package structure

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/theme"
)

// TestScrollRegion_ContentInset_reservesBand proves RX-1 FR-17: a scroll
// region with a ContentInset scrolls its content within the inset bounds, so
// an overlay bar in the inset band never occludes content. The viewport is the
// inset area, the first child starts at the inset top, and scrolled-to-the-end
// content never enters the inset bottom band.
func TestScrollRegion_ContentInset_reservesBand(t *testing.T) {
	sr := NewScrollRegion("Inset region")
	sr.ContentInset = marks.Const(ContentInsets{Top: 8, Right: 8, Bottom: 40, Left: 8})
	sr.SetChildren(scrollRegionVerticalChildren())

	rt := cardRuntimeStub{fonts: testkit.TestFontRegistry(t)}
	ctx := listResolvedContext(listTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	facet.Attach(sr, facet.AttachContext{Runtime: rt, Theme: ctx})
	canvas := gfx.RectFromXYWH(16, 16, 240, 160)
	measureCtx := facet.MeasureContext{
		Runtime:          rt,
		Theme:            ctx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}
	_ = sr.Layout.Measure(measureCtx, facet.Constraints{MaxSize: gfx.Size{W: canvas.Width(), H: canvas.Height()}})
	arrangeCtx := facet.ArrangeContext{
		Runtime:     rt,
		Theme:       ctx,
		ParentGroup: sr.Layout.Parent,
		ChildGroup:  sr.Layout.Child,
	}
	sr.Layout.Arrange(arrangeCtx, canvas)

	// The viewport is the inset area (top/left 8, right 8, bottom 40).
	wantView := gfx.RectFromXYWH(24, 24, 224, 112)
	if sr.cachedViewportBounds != wantView {
		t.Fatalf("viewport = %v, want inset %v", sr.cachedViewportBounds, wantView)
	}

	// The first child begins at the inset top, not the canvas top.
	first := sr.Children()[0]
	if b, ok := sr.cachedChildBounds[first.FacetID]; !ok || b.Min.Y != wantView.Min.Y {
		t.Fatalf("first child top = %v, want the inset top %v", b.Min.Y, wantView.Min.Y)
	}

	// Scrolled to the end, no child extends into the bottom inset band. Drive the
	// real onScroll input path (the offset clamps to the content end), then
	// re-arrange (InvalidateCache — the role caches the arrange for identical
	// bounds).
	sr.onScroll(facet.ScrollEvent{DeltaY: -1e9})
	sr.Layout.InvalidateCache()
	sr.Layout.Arrange(arrangeCtx, canvas)
	lastID := sr.Children()[len(sr.Children())-1].FacetID
	last := sr.cachedChildBounds[lastID]
	if last.IsEmpty() {
		t.Fatal("last child not arranged")
	}
	if last.Max.Y > wantView.Max.Y {
		t.Fatalf("last child bottom %v exceeds the inset bottom %v (FR-17 occlusion)", last.Max.Y, wantView.Max.Y)
	}
	if sr.scrollOffset.Y == 0 {
		t.Fatal("scroll offset did not advance to the content end")
	}
}

// TestScrollRegion_ContentInset_zero_default asserts the zero value of
// ContentInset keeps the pre-FR-17 behavior (the full bounds are the viewport).
func TestScrollRegion_ContentInset_zero_default(t *testing.T) {
	sr := NewScrollRegion("Plain region")
	sr.SetChildren(scrollRegionVerticalChildren())

	rt := cardRuntimeStub{fonts: testkit.TestFontRegistry(t)}
	ctx := listResolvedContext(listTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	facet.Attach(sr, facet.AttachContext{Runtime: rt, Theme: ctx})
	canvas := gfx.RectFromXYWH(16, 16, 240, 160)
	measureCtx := facet.MeasureContext{
		Runtime:          rt,
		Theme:            ctx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}
	_ = sr.Layout.Measure(measureCtx, facet.Constraints{MaxSize: gfx.Size{W: canvas.Width(), H: canvas.Height()}})
	sr.Layout.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       ctx,
		ParentGroup: sr.Layout.Parent,
		ChildGroup:  sr.Layout.Child,
	}, canvas)

	if sr.cachedViewportBounds != canvas {
		t.Fatalf("zero inset viewport = %v, want the full bounds %v", sr.cachedViewportBounds, canvas)
	}
}
