package studio

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
)

// TestE5_sourcesThreadedForStoreBoundWaves asserts the RX-1 F-dirty-source-gap
// resolution end-to-end: a store write driven through a framework mark's
// standard facet.Store subscription records a SPECIFIC invalidation source in
// the frame's dirty snapshot (not an empty source). The E6 playground's tabs
// subscribe to their ActiveIndex store via facet.Store; switching tabs records
// the mark's "tabs.ActiveIndex" source.
func TestE5_sourcesThreadedForStoreBoundWaves(t *testing.T) {
	sink := NewDirtySink(10)
	root, h := newShellWithSink(t, 1280, 800, sink)
	root.Shell().ActiveExhibit.Set(ExhibitPlayground)
	h.RunFrames(2)

	pg := root.Stage().RootFor(ExhibitPlayground).(*Playground)
	pg.ActiveTab().Set(1) // drive the tabs mark's store subscription
	h.RunFrame()

	snap, ok := sink.Latest()
	if !ok {
		t.Fatal("no snapshot captured")
	}
	tabsID := pg.Tabs().Base().ID()
	flags, present := snap.Dirty[tabsID]
	if !present {
		t.Fatalf("tabs facet %d not in the dirty wave", tabsID)
	}
	if flags&facet.DirtyLayout == 0 {
		t.Fatalf("tabs switch wave flags = %v, want DirtyLayout (the panel body re-arranges)", flags)
	}
	if src := snap.Sources[tabsID]; src != "tabs.ActiveIndex" {
		t.Fatalf("tabs source = %q, want tabs.ActiveIndex (F-dirty-source-gap)", src)
	}
}

// TestE5_feedTickSourcesRecordedWithoutShellRelayout asserts the FR-rt contract
// holds with source threading: a feed tick re-projects the E1 chart with a
// recorded source and does NOT re-lay a shell structural facet (Root/Chrome/
// Gallery) — the mark's source-recorded invalidation stays local.
func TestE5_feedTickSourcesRecordedWithoutShellRelayout(t *testing.T) {
	sink := NewDirtySink(10)
	root, h := newShellWithSink(t, 1280, 800, sink)
	e1 := shellStage(root).ActiveRoot().(*Realtime)
	shell := shellStructuralIDs(root)

	before := e1.appState.Rows.Len()
	e1.Feed().OnTick(100 * 1e6)
	h.RunUntil(func() bool { return e1.appState.Rows.Len() == before+1 }, 60)

	snap, _ := sink.Latest()
	assertNoShellLayout(t, snap, shell)
	// A store-bound facet in the wave carries a recorded source (not empty).
	recorded := 0
	for id, flags := range snap.Dirty {
		if flags&facet.DirtyProjection != 0 && snap.Sources[id] != "" {
			recorded++
		}
	}
	if recorded == 0 {
		t.Fatal("feed-tick wave recorded no invalidation sources for its store-bound facets (F-dirty-source-gap)")
	}
}
