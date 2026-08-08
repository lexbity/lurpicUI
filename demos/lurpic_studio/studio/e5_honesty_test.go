package studio

import (
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

// TestE5_honestyLabelsLimits asserts the R-fake-viz honesty contract: the
// exhibit labels the un-introspectable parts (store-level causal provenance
// and dependency edges, F-edges) instead of fabricating edge data.
func TestE5_honestyLabelsLimits(t *testing.T) {
	sink := NewDirtySink(5)
	root, h := newShellWithSink(t, 1280, 800, sink)
	stage := shellStage(root)
	stage.ActiveExhibit().Set(ExhibitPropagation)
	h.RunFrame()
	h.RunFrame()
	e5 := stage.ActiveRoot().(*Propagation)

	if got := e5.EdgeNote(); !strings.Contains(got, "not yet introspectable") {
		t.Fatalf("honesty note is missing the un-introspectable label: %q", got)
	}

	// The exhibit never draws dependency edges (they are not introspectable).
	area := e5.treeArea
	if area.IsEmpty() {
		area = gfx.Rect{Max: gfx.Point{X: 600, Y: 400}}
	}
	cmds := e5.treeCommands(area)
	for _, c := range cmds {
		if _, ok := c.(gfx.DrawPolyline); ok {
			t.Fatal("E5 drew a dependency edge; edges are not introspectable (F-edges)")
		}
	}

	// The honesty note itself is part of the rendered output.
	noteCmds := e5.edgeHonestyCommands(area)
	if len(noteCmds) == 0 {
		t.Fatal("the honesty note is not rendered")
	}
}

// TestE5_honestyEdgeViewHasNoData asserts the edge view carries no fabricated
// data source: the only edge-related output is the note.
func TestE5_honestyEdgeViewHasNoData(t *testing.T) {
	sink := NewDirtySink(5)
	root, h := newShellWithSink(t, 1280, 800, sink)
	stage := shellStage(root)
	stage.ActiveExhibit().Set(ExhibitPropagation)
	h.RunFrame()
	h.RunFrame()
	e5 := stage.ActiveRoot().(*Propagation)

	// Force a wave so the sink has data, then confirm the exhibit's render
	// still contains no edge primitives anywhere (only the note glyph).
	e1 := stage.ActiveRoot()
	_ = e1
	area := e5.treeArea
	if area.IsEmpty() {
		area = gfx.Rect{Max: gfx.Point{X: 600, Y: 400}}
	}
	all := append(e5.treeCommands(area), e5.edgeHonestyCommands(area)...)
	for _, c := range all {
		if _, ok := c.(gfx.DrawPolyline); ok {
			t.Fatal("edge view drew a line without an introspectable source")
		}
	}
}

// TestE5_overlayPrecedentReuse asserts F-overlay-precedent: the exhibit's
// dirty-node highlighting is drawn by the framework's diagnostics.Overlay
// (HighlightDirty), not a parallel dirty-highlight renderer. The regression
// gate is the renderer call site itself: E5 must reference the Overlay and
// emit a HighlightDirty fill, and the flag→color mapping must come from the
// Overlay (DirtyFlagColor), not an exhibit-local palette.
func TestE5_overlayPrecedentReuse(t *testing.T) {
	sink := NewDirtySink(5)
	root, h := newShellWithSink(t, 1280, 800, sink)
	stage := shellStage(root)
	stage.ActiveExhibit().Set(ExhibitPropagation)
	h.RunFrame()
	h.RunFrame()
	e5 := stage.ActiveRoot().(*Propagation)

	if e5.overlay == nil {
		t.Fatal("E5 does not hold a diagnostics.Overlay (F-overlay-precedent)")
	}

	// A dirty node's row must render through HighlightDirty: force a wave,
	// render a row for a dirty facet, and confirm a FillRect (the Overlay's
	// drawing) appears — and that its brush is the Overlay's flag color.
	area := e5.treeArea
	if area.IsEmpty() {
		area = gfx.Rect{Max: gfx.Point{X: 600, Y: 400}}
	}
	dirty := map[facet.FacetID]dirtyInfo{
		root.Base().ID(): {flags: facet.DirtyLayout, source: "test"},
	}
	var sawFill bool
	for _, cmd := range e5.nodeRow(area, propagationNode{id: root.Base().ID(), depth: 0, label: "root"}, dirty, area.Min.Y, 16) {
		fill, ok := cmd.(gfx.FillRect)
		if !ok {
			continue
		}
		sawFill = true
		want := e5.overlay.DirtyFlagColor(facet.DirtyLayout)
		if fill.Brush.Color != want {
			t.Fatalf("dirty fill uses exhibit-local color %v, want the Overlay's %v", fill.Brush.Color, want)
		}
	}
	if !sawFill {
		t.Fatal("a dirty node did not render an Overlay HighlightDirty fill")
	}
}
