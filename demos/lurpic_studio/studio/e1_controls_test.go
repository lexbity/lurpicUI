package studio

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
)

// rectsOverlap reports whether two rects share interior space (edge-touching
// is allowed — grid cells and stacked regions are adjacent by design).
func rectsOverlap(a, b gfx.Rect) bool {
	return a.Min.X < b.Max.X && a.Max.X > b.Min.X && a.Min.Y < b.Max.Y && a.Max.Y > b.Min.Y
}

// TestE1_controlsCard_cellsDoNotOverlap asserts RX-1 FR-15 / AC-10: every
// control in the E1 controls card occupies a unique, non-overlapping grid
// cell. The cells are unique by construction; this walk pins that no future
// placement can introduce a collision.
func TestE1_controlsCard_cellsDoNotOverlap(t *testing.T) {
	e, h := newE1Harness(t)
	_ = h
	keys := []string{"live", "chart", "color", "opacity", "grid", "max", "range"}
	rects := make([]struct {
		key  string
		rect gfx.Rect
	}, 0, len(keys))
	for _, k := range keys {
		r, ok := e.controls.ChildRect(k)
		if !ok || r.IsEmpty() {
			t.Fatalf("control %q has no arranged bounds", k)
		}
		rects = append(rects, struct {
			key  string
			rect gfx.Rect
		}{k, r})
	}
	for i := 0; i < len(rects); i++ {
		for j := i + 1; j < len(rects); j++ {
			if rectsOverlap(rects[i].rect, rects[j].rect) {
				t.Fatalf("controls card cells overlap: %q %v overlaps %q %v", rects[i].key, rects[i].rect, rects[j].key, rects[j].rect)
			}
		}
	}
}

// TestE1_bottomStrip_regionsDoNotOverlap asserts RX-1 FR-15 / AC-10: the
// controls card, jump-to-live button, radial reshape dial, feed legend, and
// table occupy non-overlapping arranged regions in the bottom strip.
func TestE1_bottomStrip_regionsDoNotOverlap(t *testing.T) {
	e, h := newE1Harness(t)
	_ = h
	regions := []struct {
		name string
		rect gfx.Rect
	}{
		{"controls", e.controls.Base().LayoutRole().ArrangedBounds},
		{"jump", e.Jump().Base().LayoutRole().ArrangedBounds},
		{"reshape", e.Reshape().Base().LayoutRole().ArrangedBounds},
		{"legend", e.legend.Base().LayoutRole().ArrangedBounds},
		{"table", e.table.Base().LayoutRole().ArrangedBounds},
	}
	for i := range regions {
		if regions[i].rect.IsEmpty() {
			t.Fatalf("bottom-strip region %q is empty", regions[i].name)
		}
	}
	for i := 0; i < len(regions); i++ {
		for j := i + 1; j < len(regions); j++ {
			if rectsOverlap(regions[i].rect, regions[j].rect) {
				t.Fatalf("bottom strip overlap: %q %v overlaps %q %v", regions[i].name, regions[i].rect, regions[j].name, regions[j].rect)
			}
		}
	}
}

// TestE1_yAxisLabels_visibleFormattedNumbers asserts RX-1 FR-15 / AC-10: the
// chart's y-axis renders its formatted numeric tick labels (>= 3 visible), not
// the clipped dash residue of the pre-FR-15 constant label column. The
// reference is the real shell (1280x800), where the plot is tall enough for
// the full tick set.
func TestE1_yAxisLabels_visibleFormattedNumbers(t *testing.T) {
	root, h := newResponsiveShell(t, 1280, 800)
	e1 := root.Stage().RootFor(ExhibitRealtime).(*Realtime)
	cmds := h.Runtime().LastOutputCommands(e1.Canvas().YAxis().Base().ID())
	labels := 0
	for _, c := range cmds {
		if _, ok := c.(gfx.DrawGlyphRun); ok {
			labels++
		}
	}
	if labels < 3 {
		t.Fatalf("y-axis rendered %d labels, want >= 3 formatted tick labels (FR-15)", labels)
	}
}

// TestE1_seriesAndGridPaintInFirstFrame asserts RX-1 FR-15 / AC-10: the chart
// paints its series in the first projected frame after data exists, and the
// grid overlay paints on demand (a ShowGrid toggle re-projects the canvas — the
// pre-FR-15 canvas never subscribed the grid store, so the grid could not
// render at all).
func TestE1_seriesAndGridPaintInFirstFrame(t *testing.T) {
	e, h := newE1Harness(t)
	e.ShowGrid().Set(true)
	h.RunFrame()

	series := h.Runtime().LastOutputCommands(e.Canvas().Line().Base().ID())
	if len(series) == 0 {
		t.Fatal("line series painted no commands in the first data frame")
	}

	canvas := h.Runtime().LastOutputCommands(e.Canvas().Base().ID())
	gridLines := 0
	for _, c := range canvas {
		if s, ok := c.(gfx.StrokePath); ok && len(s.Path.Segments) >= 2 {
			gridLines++
		}
	}
	if gridLines < 2 {
		t.Fatalf("grid painted %d spanning lines, want >= 2 with ShowGrid on", gridLines)
	}
}
