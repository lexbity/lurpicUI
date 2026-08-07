package studio

import (
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
)

// feedHost is a minimal facet that owns a Feed and exposes it to the runtime
// (jobs need a runtime to schedule and drain).
type feedHost struct {
	facet.Facet
	feed *Feed
}

func (h *feedHost) Base() *facet.Facet               { h.BindImpl(h); return &h.Facet }
func (h *feedHost) OnAttach(ctx facet.AttachContext) { h.feed.SetRuntime(ctx.Runtime) }
func (h *feedHost) OnDetach()                        {}
func (h *feedHost) OnActivate()                      {}
func (h *feedHost) OnDeactivate()                    {}

// newFeedHarness builds a feed over the app state and attaches it to a
// runtime. The warmup frame triggers the runtime's start (attach), so the
// feed's runtime services are wired before any tick.
func newFeedHarness(t *testing.T, appState *state.AppState) (*Feed, *testkit.Harness) {
	t.Helper()
	host := &feedHost{}
	host.Facet = facet.NewFacet()
	host.feed = NewFeed(appState, uint64(host.Facet.ID()))
	h := testkit.NewStandardHarness(t, 400, 300, host)
	h.RunFrame()
	return host.feed, h
}

// driveTicks submits n feed ticks (each a full cadence) and returns the id
// range the feed would insert.
func driveTicks(f *Feed, n int) {
	for i := 0; i < n; i++ {
		f.OnTick(100 * time.Millisecond)
	}
}

func TestFeed_ticksAppendRows(t *testing.T) {
	appState := state.NewAppState(seedRows(t))
	feed, h := newFeedHarness(t, appState)
	before := appState.Rows.Len()
	windowBefore := appState.LiveWindow.Get()

	driveTicks(feed, 3)
	h.RunUntil(func() bool { return appState.Rows.Len() >= before+3 }, 60)

	if got := appState.Rows.Len(); got != before+3 {
		t.Fatalf("rows = %d, want %d (3 feed rows)", got, before+3)
	}
	// The live window slid forward each tick (not paused).
	windowAfter := appState.LiveWindow.Get()
	if windowAfter[1] <= windowBefore[1] {
		t.Fatalf("live window did not slide: %v -> %v", windowBefore, windowAfter)
	}
	// The new rows carry consecutive stamped ids.
	rows := appState.Rows.All()
	first, last := rows[len(rows)-3].ID, rows[len(rows)-1].ID
	if last-first != 2 {
		t.Fatalf("feed row ids = %d..%d, want 3 consecutive", first, last)
	}
	// Regions rotate deterministically.
	regions := map[string]bool{}
	for _, r := range rows[len(rows)-3:] {
		regions[r.Region] = true
	}
	if len(regions) != 3 {
		t.Fatalf("feed regions = %v, want 3 distinct over 3 ticks", regions)
	}
}

func TestFeed_liveTailPausesWhenPaused(t *testing.T) {
	appState := state.NewAppState(seedRows(t))
	feed, h := newFeedHarness(t, appState)
	appState.Paused.Set(true)
	before := appState.Rows.Len()
	windowBefore := appState.LiveWindow.Get()

	driveTicks(feed, 2)
	h.RunUntil(func() bool { return appState.Rows.Len() >= before+2 }, 60)

	// Rows still append (the feed runs), but the live window freezes.
	if appState.Rows.Len() != before+2 {
		t.Fatalf("rows = %d, want %d", appState.Rows.Len(), before+2)
	}
	if window := appState.LiveWindow.Get(); window != windowBefore {
		t.Fatalf("paused live window moved: %v -> %v", windowBefore, window)
	}
}

// TestFeed_trimToMaxLongRun is the F-collection-evict proof: driving far more
// ticks than MaxRows never exceeds the cap, and the evicted rows are always
// the oldest by id.
func TestFeed_trimToMaxLongRun(t *testing.T) {
	appState := state.NewAppState(seedRows(t))
	appState.MaxRows = 20
	feed, h := newFeedHarness(t, appState)

	seedCount := appState.Rows.Len()
	const ticks = 60
	driveTicks(feed, ticks)
	// Wait until the last feed row (the highest id) has committed and trimmed.
	h.RunUntil(func() bool {
		rows := appState.Rows.All()
		return len(rows) > 0 && rows[len(rows)-1].ID == uint64(seedCount+ticks)
	}, 300)
	h.RunFrames(20)

	got := appState.Rows.Len()
	if got > appState.MaxRows {
		t.Fatalf("rows = %d, exceed MaxRows %d", got, appState.MaxRows)
	}
	// The survivors are the newest rows (highest ids).
	rows := appState.Rows.All()
	if len(rows) > 0 {
		wantFirstID := uint64(seedCount + ticks - len(rows) + 1)
		if rows[0].ID != wantFirstID {
			t.Fatalf("oldest surviving id = %d, want %d (oldest evicted)", rows[0].ID, wantFirstID)
		}
	}
}

func TestFeed_jobProgressPulses(t *testing.T) {
	appState := state.NewAppState(seedRows(t))
	feed, h := newFeedHarness(t, appState)

	feed.OnTick(100 * time.Millisecond)
	if got := feed.JobProgress.Get(); got != 0 {
		t.Fatalf("JobProgress after submit = %v, want 0", got)
	}
	h.RunUntil(func() bool { return feed.JobProgress.Get() == 1.0 }, 60)
	// The next submit resets it.
	feed.OnTick(100 * time.Millisecond)
	if got := feed.JobProgress.Get(); got != 0 {
		t.Fatalf("JobProgress after next submit = %v, want 0 (reset)", got)
	}
}

func TestFeed_cancelNoLeak(t *testing.T) {
	testkit.CheckNoLeaks(t)
	appState := state.NewAppState(seedRows(t))
	feed, h := newFeedHarness(t, appState)
	driveTicks(feed, 5)
	h.RunUntil(func() bool { return appState.Rows.Len() >= 40+5 }, 60)
	feed.Cancel()
	h.RunFrames(5)
}
