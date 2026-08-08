package studio

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/platform"
)

// chartPointScreen returns the screen position of the given row's chart point
// (the canvas is arranged at the surface origin, so its local space is the
// screen space).
func chartPointScreen(t *testing.T, e *Realtime, rowIdx int) gfx.Point {
	t.Helper()
	xs := e.Canvas().XScale().Get()
	ys := e.Canvas().YScale().Get()
	plot := e.Canvas().PlotRect()
	if plot.IsEmpty() {
		t.Fatal("plot is not arranged")
	}
	row := e.appState.Rows.All()[rowIdx]
	return gfx.Point{
		X: plot.Min.X + float32(xs.Map(vizRowTime(row))),
		Y: plot.Min.Y + float32(ys.Map(vizRowValue(row))),
	}
}

// expandLiveWindow widens the live tail to span the seed and removes the
// YAxisMax clamp so the seed rows land on-screen (the default 60s live tail +
// default clamp put the seed outside the visible plot).
func expandLiveWindow(t *testing.T, e *Realtime) {
	t.Helper()
	e.appState.WindowSeconds.Set(40 * 24 * 3600)
	e.appState.YAxisMax.Set(0)
	last := e.appState.Rows.All()[e.appState.Rows.Len()-1]
	e.appState.AnchorLiveWindow(float64(last.Time.Unix()))
}

// settleChart runs two frames after a chart config change: the first
// propagates the reactive scales (derived → bridge → scale), the second
// re-projects the marks and rebuilds the hit map used for pointer routing.
func settleChart(h *testkit.Harness) {
	h.RunFrame()
	h.RunFrame()
}

// TestBrush_gridRowClickSelectsChartPoint drives grid → chart: clicking a
// spreadsheet row publishes the Selection store, which the chart draws as a
// highlight over that row's point.
func TestBrush_gridRowClickSelectsChartPoint(t *testing.T) {
	e, h := newE1Harness(t)
	e.Canvas().ChartTypeStore().Set("point")
	h.RunFrame()

	row := e.appState.Rows.All()[0]
	want := e.appState.Rows.Identify(row)
	driveClick(h, gridValueCellPoint(t, e, 0))

	if got := e.Brush().Selection.Get(); got != want {
		t.Fatalf("row click selection = %v, want %v", got, want)
	}
}

// TestBrush_chartPointHoverHighlightsRow drives chart → grid: hovering a chart
// point publishes the Hover store, and the grid highlights (and scrolls into
// view) that row.
func TestBrush_chartPointHoverHighlightsRow(t *testing.T) {
	e, h := newE1Harness(t)
	expandLiveWindow(t, e)
	e.Canvas().ChartTypeStore().Set("point")
	settleChart(h)

	rowIdx := e.appState.Rows.Len() - 1 // the newest row, inside the live window
	row := e.appState.Rows.All()[rowIdx]
	want := e.appState.Rows.Identify(row)

	h.InjectEvent(platform.EventPointer{Kind: platform.PointerMove, Position: chartPointScreen(t, e, rowIdx)})
	h.RunFrame()

	if got := e.Brush().Hover.Get(); got == nil || *got != want {
		t.Fatalf("chart hover = %v, want %v", got, want)
	}
	// The grid scrolled the hovered row into view.
	scroll := e.Grid().ScrollOffset()
	visible := int(e.Grid().Base().LayoutRole().ArrangedBounds.Height()/gridRowHeight) + 1
	if rowIdx < scroll || rowIdx >= scroll+visible {
		t.Fatalf("hovered row %d not in view (scroll=%d visible=%d)", rowIdx, scroll, visible)
	}
}

// TestBrush_chartBarHoverHighlightsRegion drives bar → grid: hovering a bar
// band publishes the region-group sentinel, and the grid highlights every row
// of that region.
func TestBrush_chartBarHoverHighlightsRegion(t *testing.T) {
	e, h := newE1Harness(t)
	expandLiveWindow(t, e)
	e.Canvas().ChartTypeStore().Set("bar")
	settleChart(h)

	pt := chartBarBandScreen(t, e)
	h.InjectEvent(platform.EventPointer{Kind: platform.PointerMove, Position: pt})
	h.RunFrame()

	if got := e.Brush().HoverRegion.Get(); got == "" {
		t.Fatal("bar hover did not publish a region")
	}
	if got := e.Brush().Hover.Get(); got == nil || *got != regionHoverSentinel {
		t.Fatalf("bar hover id = %v, want the region sentinel", got)
	}
	// Every in-view row of that region is highlighted in the grid.
	region := e.Brush().HoverRegion.Get()
	scroll := e.Grid().ScrollOffset()
	visible := int(e.Grid().Base().LayoutRole().ArrangedBounds.Height()/gridRowHeight) + 1
	highlighted := 0
	for i, r := range e.appState.Rows.All() {
		if r.Region != region || i < scroll || i >= scroll+visible {
			continue
		}
		if rr := e.Grid().RowRect(e.appState.Rows.Identify(r)); rr.IsEmpty() {
			t.Fatalf("row %d (%s) not highlighted", r.ID, r.Region)
		}
		highlighted++
	}
	if highlighted == 0 {
		t.Fatalf("no rows of region %q are visible to highlight", region)
	}
}

// TestBrush_clearOnEmptyPlot asserts hovering empty chart area clears the
// hover (no stale highlight).
func TestBrush_clearOnEmptyPlot(t *testing.T) {
	e, h := newE1Harness(t)
	expandLiveWindow(t, e)
	e.Canvas().ChartTypeStore().Set("point")
	settleChart(h)

	// Move over a point to establish a hover.
	h.InjectEvent(platform.EventPointer{Kind: platform.PointerMove, Position: chartPointScreen(t, e, 0)})
	h.RunFrame()
	if e.Brush().Hover.Get() == nil {
		t.Fatal("hover not established")
	}

	// Move to a spot clear of any point (top-left corner of the plot).
	plot := e.Canvas().PlotRect()
	empty := gfx.Point{X: plot.Min.X + 2, Y: plot.Min.Y + 2}
	h.InjectEvent(platform.EventPointer{Kind: platform.PointerMove, Position: empty})
	h.RunFrame()

	if e.Brush().Hover.Get() != nil {
		t.Fatalf("hover not cleared on empty plot: %v", *e.Brush().Hover.Get())
	}
}

func TestBrush_selectedRowIDIsStableAcrossEdits(t *testing.T) {
	e, h := newE1Harness(t)
	row := e.appState.Rows.All()[0]
	id := e.appState.Rows.Identify(row)

	// Select through the grid, then commit an edit on that same row: the
	// selection id stays valid (the row id is the collection identity).
	activateCell(t, h, e, 0)
	e.Grid().CellValue().Set("321")
	driveKey(h, platform.KeyEnter)

	if got := e.Brush().Selection.Get(); got != id {
		t.Fatalf("selection drifted on edit: %v, want %v", got, id)
	}
}

// chartBarBandScreen returns a screen position inside one of the bar chart's
// bands.
func chartBarBandScreen(t *testing.T, e *Realtime) gfx.Point {
	t.Helper()
	cmds := e.Canvas().Bar().Base().ProjectionRole().Project(facet.ProjectionContext{
		Bounds:       e.Canvas().Bar().Base().LayoutRole().ArrangedBounds,
		ContentScale: 1,
	})
	plot := e.Canvas().PlotRect()
	for _, c := range cmds.Commands {
		if f, ok := c.(gfx.FillRect); ok && !f.Rect.IsEmpty() {
			// Bars grow from a baseline that can sit below the plot, so probe
			// a point that is inside both a band and the canvas's plot area.
			pt := gfx.Point{X: f.Rect.Min.X + f.Rect.Width()/2, Y: f.Rect.Min.Y + 2}
			if plot.Contains(pt) {
				return pt
			}
		}
	}
	t.Fatal("no bar band rendered inside the plot")
	return gfx.Point{}
}
