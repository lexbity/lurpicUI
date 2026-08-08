package studio

import (
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/platform"
)

// gridValueCellPoint returns the screen point of the given row's Value cell
// (rowIdx is relative to the grid's visible viewport).
func gridValueCellPoint(t *testing.T, e *Realtime, rowIdx int) gfx.Point {
	t.Helper()
	bounds := e.Grid().Base().LayoutRole().ArrangedBounds
	if bounds.IsEmpty() {
		t.Fatal("grid is not arranged")
	}
	x := bounds.Min.X + bounds.Width()*(gridColumns[1].start+gridColumns[1].end)/2
	y := bounds.Min.Y + float32(rowIdx)*gridRowHeight
	return gfx.Point{X: x, Y: y}
}

func driveClick(h *testkit.Harness, pt gfx.Point) {
	h.InjectEvent(platform.EventPointer{Kind: platform.PointerPress, Position: pt, Button: platform.PointerLeft})
	h.InjectEvent(platform.EventPointer{Kind: platform.PointerRelease, Position: pt, Button: platform.PointerLeft})
	h.RunFrame()
}

func driveKey(h *testkit.Harness, key platform.Key) {
	h.InjectEvent(platform.EventKey{Kind: platform.KeyPress, Key: key})
	h.InjectEvent(platform.EventKey{Kind: platform.KeyRelease, Key: key})
	h.RunFrame()
}

// activateCell clicks the given row's Value cell to open the editor.
func activateCell(t *testing.T, h *testkit.Harness, e *Realtime, rowIdx int) {
	t.Helper()
	driveClick(h, gridValueCellPoint(t, e, rowIdx))
	if !e.Grid().Editing() {
		t.Fatalf("clicking a Value cell did not open the editor (editing=%v)", e.Grid().Editing())
	}
}

// TestGrid_editCommitsToRowsAndRecharts drives the full edit path through the
// runtime (which runs on the runtime thread): activate a Value cell, set the
// editor's streamed value, press Enter to commit, and assert Rows.Update fired
// and the chart re-projected the same frame.
func TestGrid_editCommitsToRowsAndRecharts(t *testing.T) {
	e, h := newE1Harness(t)
	// Widen the live window so the edited (oldest) seed row is visible to the
	// windowed chart; otherwise the edit cannot move a projected point.
	expandLiveWindow(t, e)
	settleChart(h)
	row := e.appState.Rows.All()[0]
	id := e.appState.Rows.Identify(row)
	old := row.Value
	pointsBefore := linePoints(t, e)

	activateCell(t, h, e, 0)
	e.Grid().CellValue().Set("123")
	driveKey(h, platform.KeyEnter)

	updated, ok := e.appState.Rows.Get(id)
	if !ok {
		t.Fatal("edited row vanished")
	}
	if updated.Value != 123 {
		t.Fatalf("Rows.Update not applied: value = %v, want 123", updated.Value)
	}
	if updated.Value == old {
		t.Fatalf("value unchanged (%v)", old)
	}
	// Enter commits and advances: the editor stays open on the next cell.
	if !e.Grid().Editing() {
		t.Fatal("Enter closed the editor; it should advance to the next cell")
	}
	if got := e.Grid().EditRow(); got == id {
		t.Fatal("Enter did not advance the editor off the committed cell")
	}

	// The chart re-projected the edited row (the point moved).
	pointsAfter := linePoints(t, e)
	if len(pointsAfter) != len(pointsBefore) {
		t.Fatalf("line point count changed on edit: %d -> %d", len(pointsBefore), len(pointsAfter))
	}
	different := false
	for i := range pointsAfter {
		if pointsAfter[i] != pointsBefore[i] {
			different = true
			break
		}
	}
	if !different {
		t.Fatal("chart did not re-project the edited row")
	}
}

// TestGrid_invalidCommitShowsAlertNoWrite asserts the invalid-numeric path:
// the inline alert is shown and the store is untouched.
func TestGrid_invalidCommitShowsAlertNoWrite(t *testing.T) {
	e, h := newE1Harness(t)
	row := e.appState.Rows.All()[0]
	id := e.appState.Rows.Identify(row)

	activateCell(t, h, e, 0)
	e.Grid().CellValue().Set("not-a-number")
	driveKey(h, platform.KeyEnter)

	if got := e.Grid().Invalid().Get(); got == "" {
		t.Fatal("invalid commit did not raise the inline alert")
	}
	if updated, ok := e.appState.Rows.Get(id); !ok || updated.Value != row.Value {
		t.Fatalf("invalid commit wrote the store: %v", updated.Value)
	}
	if !e.Grid().Editing() {
		t.Fatal("invalid commit closed the editor; it should stay open for correction")
	}
}

// TestGrid_escapeCancels asserts the Escape path restores the cell and closes
// the editor without writing.
func TestGrid_escapeCancels(t *testing.T) {
	e, h := newE1Harness(t)
	row := e.appState.Rows.All()[0]
	id := e.appState.Rows.Identify(row)

	activateCell(t, h, e, 0)
	e.Grid().CellValue().Set("999")
	driveKey(h, platform.KeyEscape)

	if e.Grid().Editing() {
		t.Fatal("Escape did not close the editor")
	}
	if updated, ok := e.appState.Rows.Get(id); !ok || updated.Value != row.Value {
		t.Fatalf("Escape wrote the store: %v", updated.Value)
	}
}

// TestGrid_enterCommitsAndAdvances asserts Enter commits the current cell and
// moves the editor to the next row's Value cell. (Cell traversal is Enter-
// driven: F-tab-eaten — the input system consumes Tab for global focus
// traversal before a focused facet observes it, so a spreadsheet Tab-traverse
// cannot reach the grid through the standard key path.)
func TestGrid_enterCommitsAndAdvances(t *testing.T) {
	e, h := newE1Harness(t)
	rows := e.appState.Rows.All()
	firstID := e.appState.Rows.Identify(rows[0])
	secondID := e.appState.Rows.Identify(rows[1])

	activateCell(t, h, e, 0)
	e.Grid().CellValue().Set("55")
	driveKey(h, platform.KeyEnter)

	if updated, ok := e.appState.Rows.Get(firstID); !ok || updated.Value != 55 {
		t.Fatalf("Enter did not commit the current cell: %v", updated.Value)
	}
	if !e.Grid().Editing() {
		t.Fatal("Enter did not keep the editor open for the next cell")
	}
	if got := e.Grid().EditRow(); got != secondID {
		t.Fatalf("Enter did not advance to the next row (editRow %v, want %v)", got, secondID)
	}
}

// TestGrid_editCommitsOnRuntimeThread drives the commit through the harness
// (the runtime thread) to lock in the thread contract: Rows.Update must run on
// the runtime thread. Under -tags lurpic_debug the CollectionStore asserts
// this; on the default build the assertion is compiled out, so the test's
// value is proving the commit path executes through the runtime.
func TestGrid_editCommitsOnRuntimeThread(t *testing.T) {
	e, h := newE1Harness(t)
	activateCell(t, h, e, 0)
	e.Grid().CellValue().Set("77")
	driveKey(h, platform.KeyEnter)

	if updated, ok := e.appState.Rows.Get(e.appState.Rows.Identify(e.appState.Rows.All()[0])); !ok || updated.Value != 77 {
		t.Fatalf("commit did not apply on the runtime thread: %v", updated.Value)
	}
}

// TestGrid_rowsFollowTheCollection exercises the CollectionBinder over Rows
// (F-unconsumed): a feed insert grows the grid's row facets and re-arranges
// them.
func TestGrid_rowsFollowTheCollection(t *testing.T) {
	e, h := newE1Harness(t)
	before := len(e.Grid().binder.Children())

	e.Feed().OnTick(100 * time.Millisecond)
	h.RunUntil(func() bool { return len(e.Grid().binder.Children()) == before+1 }, 60)

	if got := len(e.Grid().binder.Children()); got != before+1 {
		t.Fatalf("grid rows = %d, want %d (binder did not follow the insert)", got, before+1)
	}
	if got := e.Grid().binder.Children()[0].Base().State(); got != facet.StateActive {
		t.Fatalf("binder row state = %v, want active", got)
	}
}
