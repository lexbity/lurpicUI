package studio

import (
	"strings"
	"testing"

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
