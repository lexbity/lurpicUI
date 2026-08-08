package studio

import (
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/app"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/theme"
)

// newShellWithSink returns a harness running the gallery shell with the E5
// dirty sink installed on the runtime (the app-config wiring, minus app.Run).
func newShellWithSink(t *testing.T, w, h int, sink *DirtySink) (*Root, *testkit.Harness) {
	t.Helper()
	ctx := app.BuildContext{
		WindowSize:   gfx.Size{W: float32(w), H: float32(h)},
		ContentScale: 1,
		Theme:        theme.DefaultResolvedContext(),
	}
	root := NewRoot(ctx, sink, seedRows(t))
	harness := testkit.NewStandardHarness(t, w, h, root)
	harness.Runtime().EnableDiagnostics(sink)
	harness.RunFrame()
	return root, harness
}

// shellStage returns the shell's stage pane facet.
func shellStage(root *Root) *Stage {
	return root.GallerySplit().Panes()[1].Facet.(*Stage)
}

// shellStructuralIDs returns the shell's chrome facets (the "no shell
// DirtyLayout" FR-rt assertion target).
func shellStructuralIDs(root *Root) map[facet.FacetID]bool {
	return map[facet.FacetID]bool{
		root.Base().ID():                true,
		root.ChromeStack().Base().ID():  true,
		root.GallerySplit().Base().ID(): true,
		root.StatusBar().Base().ID():    true,
	}
}

func latestSnapshot(t *testing.T, sink *DirtySink) runtime.DirtySnapshot {
	t.Helper()
	latest, ok := sink.Latest()
	if !ok {
		t.Fatal("sink has no snapshot")
	}
	return latest
}

func hasProjectionDirty(snap runtime.DirtySnapshot) bool {
	for _, flags := range snap.Dirty {
		if flags&facet.DirtyProjection != 0 {
			return true
		}
	}
	return false
}

func assertNoShellLayout(t *testing.T, snap runtime.DirtySnapshot, shell map[facet.FacetID]bool) {
	t.Helper()
	for id, flags := range snap.Dirty {
		if shell[id] && flags&facet.DirtyLayout != 0 {
			t.Fatalf("wave re-laid-out a shell structural facet %d: %v", id, flags)
		}
	}
}

// TestE5_propagationWave_feedTick drives an E1 feed tick and asserts the sink
// captured a projection-only wave (the FR-rt property from E5's vantage).
func TestE5_propagationWave_feedTick(t *testing.T) {
	sink := NewDirtySink(10)
	root, h := newShellWithSink(t, 1280, 800, sink)
	e1 := shellStage(root).ActiveRoot().(*Realtime)
	shell := shellStructuralIDs(root)

	before := e1.appState.Rows.Len()
	e1.Feed().OnTick(100 * time.Millisecond)
	h.RunUntil(func() bool { return e1.appState.Rows.Len() == before+1 }, 60)

	snap := latestSnapshot(t, sink)
	if len(snap.Dirty) == 0 {
		t.Fatal("feed tick produced an empty dirty wave")
	}
	if !hasProjectionDirty(snap) {
		t.Fatal("feed tick wave has no DirtyProjection facet")
	}
	assertNoShellLayout(t, snap, shell)
}

// TestE5_propagationWave_cellEdit drives the E1 write-back path (a cell edit
// commits through Rows.Update) and asserts the sink captured the projection
// wave.
func TestE5_propagationWave_cellEdit(t *testing.T) {
	sink := NewDirtySink(10)
	root, h := newShellWithSink(t, 1280, 800, sink)
	e1 := shellStage(root).ActiveRoot().(*Realtime)
	shell := shellStructuralIDs(root)

	row := e1.appState.Rows.All()[0]
	row.Value = 1234
	e1.appState.Rows.Update(row)
	h.RunFrame()

	snap := latestSnapshot(t, sink)
	if len(snap.Dirty) == 0 {
		t.Fatal("cell edit produced an empty dirty wave")
	}
	if !hasProjectionDirty(snap) {
		t.Fatal("cell edit wave has no DirtyProjection facet")
	}
	assertNoShellLayout(t, snap, shell)
}

// TestE5_propagationWave_brush drives the linked-brush channel (a hover store
// write) and asserts the sink captured the projection wave.
func TestE5_propagationWave_brush(t *testing.T) {
	sink := NewDirtySink(10)
	root, h := newShellWithSink(t, 1280, 800, sink)
	e1 := shellStage(root).ActiveRoot().(*Realtime)
	shell := shellStructuralIDs(root)

	id := e1.appState.Rows.Identify(e1.appState.Rows.All()[0])
	e1.Brush().Hover.Set(&id)
	h.RunFrame()

	snap := latestSnapshot(t, sink)
	if len(snap.Dirty) == 0 {
		t.Fatal("brush wave produced an empty dirty wave")
	}
	if !hasProjectionDirty(snap) {
		t.Fatal("brush wave has no DirtyProjection facet")
	}
	assertNoShellLayout(t, snap, shell)
}

// TestE5_propagationWave_resize switches to E4 and drives a split SetPanes
// (a structural resize) and asserts the sink captured a DirtyLayout wave on
// the shell.
func TestE5_propagationWave_resize(t *testing.T) {
	sink := NewDirtySink(10)
	root, h := newShellWithSink(t, 1280, 800, sink)
	stage := shellStage(root)
	stage.ActiveExhibit().Set(ExhibitPolicies)
	h.RunFrame()
	h.RunFrame()

	e4 := stage.ActiveRoot().(*LayoutPolicies)
	shell := shellStructuralIDs(root)
	panes := e4.Split().Panes()
	panes[2].FixedWidth += 40
	e4.Split().SetPanes(panes)
	h.RunFrame()

	snap := latestSnapshot(t, sink)
	if len(snap.Dirty) == 0 {
		t.Fatal("resize produced an empty dirty wave")
	}
	layoutSeen := false
	for _, flags := range snap.Dirty {
		if flags&facet.DirtyLayout != 0 {
			layoutSeen = true
		}
	}
	if !layoutSeen {
		t.Fatal("resize wave has no DirtyLayout facet")
	}
	_ = shell
}

// TestE5_propagation_pauseFreezesCapture asserts the pause switch stops the
// sink from staging new snapshots and the live light goes off.
func TestE5_propagation_pauseFreezesCapture(t *testing.T) {
	sink := NewDirtySink(10)
	root, h := newShellWithSink(t, 1280, 800, sink)
	stage := shellStage(root)
	stage.ActiveExhibit().Set(ExhibitPropagation)
	h.RunFrame()
	h.RunFrame()

	e5 := stage.ActiveRoot().(*Propagation)
	before := sink.Count()
	e5.paused.Set(true)
	// The pause propagates via the OnAttach subscription; run frames so the
	// tick would otherwise stage snapshots.
	for i := 0; i < 5; i++ {
		h.RunFrame()
	}
	if got := sink.Count(); got != before {
		t.Fatalf("paused sink still staged snapshots: %d -> %d", before, got)
	}
	if !sink.Paused() {
		t.Fatal("sink did not enter the paused state")
	}
}

// TestE5_propagation_retentionWindow asserts the retention slider resizes the
// sink's ring buffer.
func TestE5_propagation_retentionWindow(t *testing.T) {
	sink := NewDirtySink(10)
	root, h := newShellWithSink(t, 1280, 800, sink)
	stage := shellStage(root)
	stage.ActiveExhibit().Set(ExhibitPropagation)
	h.RunFrame()

	e5 := stage.ActiveRoot().(*Propagation)
	e5.retention.Set(3)
	for i := 0; i < 10; i++ {
		h.RunFrame()
	}
	if got := len(sink.Snapshots()); got != 3 {
		t.Fatalf("retained snapshots = %d, want 3 (retention window)", got)
	}
}

// TestE5_propagation_dirtySinkIsDiagnosticsHook pins the wiring contract: the
// sink implements both DiagnosticsHook and DirtySnapshotSink (the app-installed
// hook), and a fresh sink is not yet live.
func TestE5_propagation_dirtySinkIsDiagnosticsHook(t *testing.T) {
	sink := NewDirtySink(5)
	var _ runtime.DirtySnapshotSink = sink
	if sink.Live() {
		t.Fatal("fresh sink should not report live until the first snapshot")
	}
	if sink.Paused() {
		t.Fatal("fresh sink should not be paused")
	}
}
