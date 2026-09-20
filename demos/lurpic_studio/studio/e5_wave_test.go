package studio

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
)

// driveE5Waves drives a few real shell dirty waves so the E5 sink carries
// dirty state for the wave map to highlight.
func driveE5Waves(t *testing.T, root *Root, h interface{ RunFrame() }) {
	t.Helper()
	root.Shell().Compact.Set(true)
	h.RunFrame()
	root.Shell().Compact.Set(false)
	h.RunFrame()
	root.Shell().CommandOpen.Set(true)
	h.RunFrame()
	root.Shell().CommandOpen.Set(false)
	h.RunFrame()
}

// TestE5_waveView_labelCapAndCulling is the RX-1 FR-16 / AC-11 net: the wave
// view renders at most waveLabelCap labels, no label lands outside the plot
// rect, and every in-area node renders a bounds-scaled rectangle. This is the
// regression gate for the A-11 label explosion.
func TestE5_waveView_labelCapAndCulling(t *testing.T) {
	sink := NewDirtySink(10)
	root, h := newShellWithSink(t, 1280, 800, sink)
	stage := shellStage(root)
	stage.ActiveExhibit().Set(ExhibitPropagation)
	h.RunFrame()
	h.RunFrame()

	e5 := stage.ActiveRoot().(*Propagation)
	area := e5.TreeArea()
	if area.IsEmpty() {
		t.Fatal("wave view area not arranged")
	}
	driveE5Waves(t, root, h)

	cmds := e5.treeCommands(area)

	// Labels: capped and inside the plot rect.
	labels := 0
	for _, c := range cmds {
		g, ok := c.(gfx.DrawGlyphRun)
		if !ok {
			continue
		}
		labels++
		if g.Origin.X < area.Min.X || g.Origin.X > area.Max.X ||
			g.Origin.Y < area.Min.Y || g.Origin.Y > area.Max.Y {
			t.Fatalf("label at %v outside the plot rect %v", g.Origin, area)
		}
	}
	if labels > waveLabelCap {
		t.Fatalf("wave view rendered %d labels, want <= %d (AC-11)", labels, waveLabelCap)
	}

	// Nodes: every in-area node emits a bounds-scaled rect (a fill + an edge),
	// so the map is never blank for arranged facets.
	shell := e5.shellExtent()
	if shell.IsEmpty() {
		t.Fatal("wave map has no arranged shell geometry")
	}
	scale := e5.mapScale(area, shell)
	base := e5.mapOffset(area, shell, scale)
	wantRects := 0
	for _, n := range e5.tree {
		r := e5.nodeRect(base, scale, n.bounds)
		if r.Width() < 2 || r.Height() < 2 {
			continue
		}
		if r.Max.X <= area.Min.X || r.Min.X >= area.Max.X ||
			r.Max.Y <= area.Min.Y || r.Min.Y >= area.Max.Y {
			continue
		}
		wantRects++
	}
	if wantRects == 0 {
		t.Fatal("no in-area nodes; the wave map is empty")
	}
	rects := 0
	for _, c := range cmds {
		switch c.(type) {
		case gfx.FillRect, gfx.StrokeRect:
			rects++
		}
	}
	if rects < wantRects*2 {
		t.Fatalf("node rect commands = %d, want >= %d (fill+edge per in-area node)", rects, wantRects*2)
	}
}

// TestE5_waveView_frameLabelBound extends the cap to the projected frame: the
// E5 facet's own output (the wave map + legend + honesty note) carries at most
// the tree cap plus the fixed legend/note glyphs — a frame-level no-explosion
// gate on the actual render path.
func TestE5_waveView_frameLabelBound(t *testing.T) {
	sink := NewDirtySink(10)
	root, h := newShellWithSink(t, 1280, 800, sink)
	stage := shellStage(root)
	stage.ActiveExhibit().Set(ExhibitPropagation)
	h.RunFrame()
	h.RunFrame()

	e5 := stage.ActiveRoot().(*Propagation)
	area := e5.TreeArea()
	if area.IsEmpty() {
		t.Fatal("wave view area not arranged")
	}
	driveE5Waves(t, root, h)
	h.RunFrame()

	frame := h.Runtime().LastOutputCommands(e5.Base().ID())
	labels := 0
	for _, c := range frame {
		g, ok := c.(gfx.DrawGlyphRun)
		if !ok {
			continue
		}
		labels++
		if g.Origin.X < area.Min.X || g.Origin.X > area.Max.X ||
			g.Origin.Y < area.Min.Y || g.Origin.Y > area.Max.Y {
			t.Fatalf("frame label at %v outside the plot rect %v", g.Origin, area)
		}
	}
	// Tree cap + the fixed legend labels + the edge-honesty note.
	if labels > waveLabelCap+8 {
		t.Fatalf("frame rendered %d labels, want <= %d (tree cap + legend + note)", labels, waveLabelCap+8)
	}
}
