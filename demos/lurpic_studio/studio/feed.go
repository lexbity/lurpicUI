package studio

import (
	"math"
	"time"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/store"
)

// DefaultFeedCadence is the initial row cadence.
const DefaultFeedCadence = 100 * time.Millisecond

// feedRegions rotates through the categorical regions so the bar chart has all
// four bands over a run.
var feedRegions = []string{"north", "south", "east", "west"}

// Feed drives the streaming row source. Every cadence it submits a
// snapshot→work→commit job: the worker goroutine generates one deterministic
// Row (pure — never touches a store), and the runtime-thread commit appends
// it, slides the live window (unless paused), and trims the collection
// (F-collection-evict). The feed is cancelable and leaves no goroutines behind
// (the runtime's job pool is cancelled on shutdown).
type Feed struct {
	appState *state.AppState
	cadence  *store.ValueStore[time.Duration]
	live     *store.ValueStore[bool]

	// JobProgress pulses 0 → 1 per committed job (the status bar tracks it in
	// lock-step).
	JobProgress *store.ValueStore[float64]

	synthetic time.Time
	elapsed   time.Duration
	tickCount int
	jobSeq    job.JobID
	rt        facet.RuntimeServices
	ownerID   uint64
}

// NewFeed builds a feed over the shared app state. The synthetic clock starts
// just past the seed window's end so the live tail includes the recent seed
// rows. SetRuntime must be called once attached.
func NewFeed(appState *state.AppState, ownerID uint64) *Feed {
	hi := appState.LiveWindow.Get()[1]
	start := time.Unix(int64(hi), 0)
	return &Feed{
		appState:    appState,
		cadence:     store.NewValueStore(DefaultFeedCadence),
		live:        store.NewValueStore(true),
		JobProgress: store.NewValueStore(0.0),
		synthetic:   start.Add(DefaultFeedCadence),
		ownerID:     ownerID,
	}
}

// SetRuntime wires the runtime services (called from the owning facet's
// OnAttach).
func (f *Feed) SetRuntime(rt facet.RuntimeServices) { f.rt = rt }

// Cadence returns the row-cadence store.
func (f *Feed) Cadence() *store.ValueStore[time.Duration] { return f.cadence }

// Live returns the feed gate store.
func (f *Feed) Live() *store.ValueStore[bool] { return f.live }

// Cancel cancels the most recently submitted, still-pending job (a no-op once
// it has committed).
func (f *Feed) Cancel() {
	if f.rt != nil && f.jobSeq != 0 {
		f.rt.CancelJob(f.jobSeq)
	}
}

// OnTick advances the cadence accumulator and submits a feed job when the
// interval elapses. It runs on the runtime thread via the owning facet's
// TickRole.
func (f *Feed) OnTick(dt time.Duration) {
	if !f.live.Get() {
		f.elapsed = 0
		return
	}
	f.elapsed += dt
	if f.elapsed < f.cadence.Get() {
		return
	}
	f.elapsed = 0
	f.submitTick()
}

func (f *Feed) submitTick() {
	now := f.synthetic
	f.synthetic = f.synthetic.Add(f.cadence.Get())
	f.JobProgress.Set(0)
	f.tickCount++
	f.jobSeq++
	tickIndex := f.tickCount // captured per-submit so concurrent workers agree
	appState := f.appState
	interval := f.cadence.Get()
	snap := job.NewSnapshot(float64(now.UnixNano())/1e9, appState.Rows.Version())
	aj := job.BindJob(f.ownerID, job.Job[float64, dataset.Row]{
		ID:       f.jobSeq,
		Priority: job.PriorityInteractive,
		Snapshot: snap,
		Work: func(s job.Snapshot[float64], cancel *job.CancelToken) (dataset.Row, error) {
			return generateRow(unixToTime(s.Data), interval, tickIndex), nil
		},
	}, func(row dataset.Row) {
		appState.InsertRow(row)
		if !appState.Paused.Get() {
			appState.AnchorLiveWindow(float64(row.Time.UnixNano()) / 1e9)
		}
		appState.TrimToMax()
		f.JobProgress.Set(1.0)
	})
	if f.rt != nil {
		f.rt.Schedule(aj)
	}
}

// unixToTime reconstructs a time.Time from fractional unix seconds.
func unixToTime(f float64) time.Time {
	sec := int64(f)
	nsec := int64((f - float64(sec)) * 1e9)
	return time.Unix(sec, nsec)
}

// generateRow computes one deterministic Row at time t. The value is a
// deterministic function of t (synthetic clock — not random, NFR-determinism):
// a sinusoid around the seed's baseline plus a bounded sawtooth drift. The
// region rotates on a fixed per-tick cycle.
func generateRow(t time.Time, cadence time.Duration, tickIndex int) dataset.Row {
	unix := float64(t.UnixNano()) / 1e9
	value := 820 + 120*math.Sin(unix/300.0) + 8*math.Mod(unix/86400.0, 40.0)
	region := feedRegions[tickIndex%len(feedRegions)]
	return dataset.Row{Time: t, Value: math.Round(value*10) / 10, Region: region}
}
