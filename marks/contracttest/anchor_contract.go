package contracttest

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/theme"
)

// anchorSetsEqual reports whether two AnchorSets are exactly equal
// (same keys, same gfx.Point values). Exact equality is deliberate:
// anchors are deterministic computational results of Arrange, and
// near-equality would mask real non-determinism bugs.
func anchorSetsEqual(a, b layout.AnchorSet) bool {
	if len(a) != len(b) {
		return false
	}
	for id, pt := range a {
		bp, ok := b[id]
		if !ok || pt != bp {
			return false
		}
	}
	return true
}

// AssertAnchorExport proves that a layout.AnchorExporter mark:
//  1. after Measure+Arrange, ExportAnchors returns a non-empty AnchorSet,
//  2. every anchor Point lies inside the arranged bounds,
//  3. the anchor set is deterministic: a second Measure+Arrange on the same
//     bounds + same theme returns a deep-equal AnchorSet (map[id]Point),
//  4. dispose + rebuild reproduces the same anchor set.
//
// build:    construct the mark (no attach yet).
// arrange:  caller closure that runs Measure+Arrange at the given bounds
//
//	against the captured runtime and theme.
//
// bounds:   the arrangement bounds; anchor points must all be inside this rect.
// themeCtx: the resolved context used during arrangement.
func AssertAnchorExport(
	t TB,
	build func() facet.FacetImpl,
	arrange func(m facet.FacetImpl, ctx facet.AttachContext, bounds gfx.Rect),
	bounds gfx.Rect,
	themeCtx theme.ResolvedContext,
) {
	t.Helper()
	ctx := facet.AttachContext{Runtime: contractRuntime{}, Theme: themeCtx}

	m := build()
	facet.Attach(m, ctx)
	defer facet.Dispose(m)

	arrange(m, ctx, bounds)

	exporter, ok := m.(layout.AnchorExporter)
	if !ok {
		t.Fatalf("AssertAnchorExport: mark does not implement layout.AnchorExporter")
	}

	anchorCtx := layout.AnchorExportContext{
		ResolvedLayer: layout.ResolvedLayer{Bounds: bounds},
	}
	anchors := exporter.ExportAnchors(anchorCtx)
	if len(anchors) == 0 {
		t.Fatalf("AssertAnchorExport: ExportAnchors returned empty set after arrange")
	}

	for id, pt := range anchors {
		if !bounds.Contains(pt) {
			t.Fatalf("AssertAnchorExport: anchor %q = %v outside arranged bounds %v",
				id, pt, bounds)
		}
	}

	// (3) Determinism: second arrange produces deep-equal anchor set.
	arrange(m, ctx, bounds)
	anchors2 := exporter.ExportAnchors(anchorCtx)
	if !anchorSetsEqual(anchors, anchors2) {
		t.Fatalf("AssertAnchorExport: anchor set changed on second arrange — non-deterministic")
	}

	// (4) Rebuild: dispose + rebuild reproduces the same anchor set.
	wantAnchors := anchors
	m2 := build()
	facet.Attach(m2, ctx)
	defer facet.Dispose(m2)
	arrange(m2, ctx, bounds)
	anchors3 := m2.(layout.AnchorExporter).ExportAnchors(anchorCtx)
	if !anchorSetsEqual(wantAnchors, anchors3) {
		t.Fatalf("AssertAnchorExport: anchor set changed across dispose+rebuild")
	}
}
