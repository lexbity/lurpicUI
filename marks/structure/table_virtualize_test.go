package structure

import (
	"fmt"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme"
)

// newThousandRowTable builds a 1000-row table with stable row keys.
func newThousandRowTable() *Table {
	rows := make([]TableRow, 0, 1000)
	for i := 0; i < 1000; i++ {
		rows = append(rows, TableRow{
			Key:   fmt.Sprintf("row-%04d", i),
			Cells: []string{fmt.Sprintf("%04d", i), fmt.Sprintf("item %04d", i), "Ready"},
		})
	}
	return NewTable("Thousand rows", TableData{
		Columns: []TableColumn{
			{Key: "id", Label: "ID"},
			{Key: "name", Label: "Item"},
			{Key: "status", Label: "Status"},
		},
		Rows: rows,
	}, store.NewValueStore(""))
}

func virtualizeTestCtx(t *testing.T, table *Table, bounds gfx.Rect) (cardRuntimeStub, theme.ResolvedContext) {
	t.Helper()
	rt := cardRuntimeStub{fonts: testkit.TestFontRegistry(t)}
	ctx := listResolvedContext(listTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	facet.Attach(table, facet.AttachContext{Runtime: rt, Theme: ctx})
	table.Layout.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            ctx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: bounds.Width(), H: bounds.Height()}})
	table.Layout.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       ctx,
		ParentGroup: table.Layout.Parent,
		ChildGroup:  table.Layout.Child,
	}, bounds)
	return rt, ctx
}

func countBuiltBodyCells(t *Table) int {
	count := 0
	for _, spec := range t.cachedChildSpecs {
		if spec.MarkID == tableMarkIDBodyCell {
			count++
		}
	}
	return count
}

// TestTableVirtualize_windowBoundedToViewport pins RX-1 FR-7: with 1000 rows in
// a small viewport, only the visible window + overscan is built (measured,
// arranged, projected) — the built body-cell count is bounded by the window,
// not the row count.
func TestTableVirtualize_windowBoundedToViewport(t *testing.T) {
	table := newThousandRowTable()
	bounds := gfx.RectFromXYWH(0, 0, 480, 180)
	virtualizeTestCtx(t, table, bounds)

	start, end := table.VisibleRange()
	if start != 0 {
		t.Fatalf("VisibleRange start = %d, want 0 at the top", start)
	}
	window := end - start + 1
	if window < 1 {
		t.Fatalf("VisibleRange window = %d, want >= 1", window)
	}
	viewportRows := int(180.0 / table.cachedBodyRowHeight)
	if window > viewportRows*3+3 {
		t.Fatalf("window %d exceeds viewport(%d rows)+2*overscan", window, viewportRows)
	}
	builtCells := countBuiltBodyCells(table)
	if builtCells > window*3 {
		t.Fatalf("built body cells = %d, want <= %d (window*columns)", builtCells, window*3)
	}
	if builtCells >= 1000*3 {
		t.Fatalf("built body cells = %d, want << 3000 (not the full table)", builtCells)
	}
}

// TestTableVirtualize_scrollToBottomProjectsLastRow pins FR-7: scrolling to the
// bottom re-virtualizes so the last row is built and the window reaches the end.
func TestTableVirtualize_scrollToBottomProjectsLastRow(t *testing.T) {
	table := newThousandRowTable()
	bounds := gfx.RectFromXYWH(0, 0, 480, 180)
	virtualizeTestCtx(t, table, bounds)

	table.onScroll(facet.ScrollEvent{DeltaY: -1e6})
	table.Layout.Arrange(facet.ArrangeContext{
		Runtime:     cardRuntimeStub{fonts: testkit.TestFontRegistry(t)},
		Theme:       listResolvedContext(listTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR),
		ParentGroup: table.Layout.Parent,
		ChildGroup:  table.Layout.Child,
	}, bounds)

	_, end := table.VisibleRange()
	if end != 999 {
		t.Fatalf("VisibleRange end = %d, want 999 (last row) after scrolling to bottom", end)
	}
	lastBuilt := false
	for _, spec := range table.cachedChildSpecs {
		if spec.MarkID != tableMarkIDBodyCell {
			continue
		}
		if spec.Key == "body:"+stableTableCellKey(stableTableKey("row-0999", "", 999), stableTableKey("id", "ID", 0)) {
			lastBuilt = true
			break
		}
	}
	if !lastBuilt {
		t.Fatal("last row's cell was not built after scrolling to the bottom")
	}
}

// TestTableVirtualize_identityStableAcrossScroll pins FR-7 stable row identity:
// row keys are absolute (row.Key + absolute index), so the same row's cells are
// reused across the scroll window rather than re-keyed by window position.
func TestTableVirtualize_identityStableAcrossScroll(t *testing.T) {
	table := newThousandRowTable()
	bounds := gfx.RectFromXYWH(0, 0, 480, 180)
	virtualizeTestCtx(t, table, bounds)

	// Capture the key of a row built near the top.
	mid := table.cachedRowWindowStart + len(table.cachedWindowRows)/2
	if mid >= len(table.cachedAllRows) {
		mid = len(table.cachedAllRows) - 1
	}
	topKey := stableTableKey(table.cachedAllRows[mid].Key, "", mid)

	table.scrollOffset.Y = float32(mid) * table.cachedBodyRowHeight
	table.Layout.Arrange(facet.ArrangeContext{
		Runtime:     cardRuntimeStub{fonts: testkit.TestFontRegistry(t)},
		Theme:       listResolvedContext(listTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR),
		ParentGroup: table.Layout.Parent,
		ChildGroup:  table.Layout.Child,
	}, bounds)

	// The same absolute key is present in the new window's row keys.
	found := false
	for _, k := range table.cachedRowKeys {
		if k == topKey {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("row key %q lost after scrolling (identity not stable)", topKey)
	}
}

// TestTableVirtualize_resizeReVirtualizes pins FR-7: a larger viewport widens
// the built window; a smaller one narrows it.
func TestTableVirtualize_resizeReVirtualizes(t *testing.T) {
	table := newThousandRowTable()
	small := gfx.RectFromXYWH(0, 0, 480, 90)
	virtualizeTestCtx(t, table, small)
	smallStart, smallEnd := table.VisibleRange()

	large := gfx.RectFromXYWH(0, 0, 480, 360)
	table.Layout.Arrange(facet.ArrangeContext{
		Runtime:     cardRuntimeStub{fonts: testkit.TestFontRegistry(t)},
		Theme:       listResolvedContext(listTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR),
		ParentGroup: table.Layout.Parent,
		ChildGroup:  table.Layout.Child,
	}, large)
	largeStart, largeEnd := table.VisibleRange()

	if (largeEnd - largeStart) <= (smallEnd - smallStart) {
		t.Fatalf("resize did not widen the window: small=%d..%d large=%d..%d", smallStart, smallEnd, largeStart, largeEnd)
	}
}
