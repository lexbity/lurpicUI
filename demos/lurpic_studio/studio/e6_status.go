package studio

import (
	"strconv"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/action"
	"codeburg.org/lexbit/lurpicui/marks/selection"
	"codeburg.org/lexbit/lurpicui/marks/status"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
)

// playStatusFamily is the Status playground: badge, status_light,
// progress_bar, progress_ring. Each indicator is store-bound, and the app's
// own write-back (a tick button, an online switch, a progress slider) lands in
// those stores and re-invalidates the mark — the status family's distinctive
// behavior: a status store reflected by an indicator.
type playStatusFamily struct {
	scroll *demoList

	badge      *status.Badge
	tick       *action.Button
	badgeLabel *store.ValueStore[string]

	light        *status.StatusLight
	online       *store.ValueStore[bool]
	lightLabel   *store.ValueStore[string]
	onlineSwitch *selection.Switch

	slider    *selection.Slider
	sliderVal *store.ValueStore[float64]
	progress  *store.ValueStore[float32]
	bar       *status.ProgressBar
	ring      *status.ProgressRing
}

// newPlayStatusFamily builds the Status family playground.
func newPlayStatusFamily() *playStatusFamily {
	f := &playStatusFamily{
		badgeLabel: store.NewValueStore("0"),
		online:     store.NewValueStore(true),
		lightLabel: store.NewValueStore("Online"),
		progress:   store.NewValueStore(float32(0.4)),
		sliderVal:  store.NewValueStore(40.0),
	}

	f.tick = action.NewButton(marks.Const("Simulate tick"), marks.Const(uiinput.ButtonText))
	f.badge = status.NewBadge("0")
	f.badge.Label = marks.FromStore(f.badgeLabel, facet.DirtyProjection)

	f.onlineSwitch = selection.NewSwitch("Connection online", f.online)
	f.light = status.NewStatusLight("Connection")
	f.light.ShowLabel = marks.Const(true)
	f.light.Label = marks.FromStore(f.lightLabel, facet.DirtyProjection)

	f.slider = selection.NewSlider("Reload throughput", 0, 100, 1, f.sliderVal)
	f.bar = status.NewProgressBar("reload")
	f.ring = status.NewProgressRing("reload")
	f.bar.Value = marks.FromStore(f.progress, facet.DirtyProjection)
	f.ring.Value = marks.FromStore(f.progress, facet.DirtyProjection)

	f.scroll = newDemoList(listGap,
		playgroundCard("badge — count events", f.tick, f.badge),
		playgroundCard("status_light — toggle online", f.onlineSwitch, f.light),
		playgroundCard("progress_bar + progress_ring — drag throughput", f.slider, f.bar, f.ring),
	)
	return f
}

func (f *playStatusFamily) wire() func() {
	tickID := f.tick.Activated.Subscribe(func(signal.Unit) {
		n := 0
		if v, err := strconv.Atoi(f.badgeLabel.Get()); err == nil {
			n = v
		}
		f.badgeLabel.Set(strconv.Itoa(n + 1))
	})
	onlineID := f.online.OnChange.Subscribe(func(signal.Change[bool]) {
		if f.online.Get() {
			f.lightLabel.Set("Online")
		} else {
			f.lightLabel.Set("Offline")
		}
		f.light.Invalidate(facet.DirtyProjection)
	})
	sliderID := f.sliderVal.OnChange.Subscribe(func(signal.Change[float64]) {
		f.progress.Set(float32(f.sliderVal.Get() / 100))
	})
	progressID := f.progress.OnChange.Subscribe(func(signal.Change[float32]) {
		f.bar.Invalidate(facet.DirtyProjection)
		f.ring.Invalidate(facet.DirtyProjection)
	})
	badgeID := f.badgeLabel.OnChange.Subscribe(func(signal.Change[string]) {
		f.badge.Invalidate(facet.DirtyProjection)
	})
	return func() {
		f.tick.Activated.Unsubscribe(tickID)
		f.online.OnChange.Unsubscribe(onlineID)
		f.sliderVal.OnChange.Unsubscribe(sliderID)
		f.progress.OnChange.Unsubscribe(progressID)
		f.badgeLabel.OnChange.Unsubscribe(badgeID)
	}
}

// StatusBadge returns the badge's label store.
func (f *playStatusFamily) BadgeLabel() *store.ValueStore[string] { return f.badgeLabel }
func (f *playStatusFamily) Online() *store.ValueStore[bool]       { return f.online }
func (f *playStatusFamily) Slider() *store.ValueStore[float64]    { return f.sliderVal }
func (f *playStatusFamily) Progress() *store.ValueStore[float32]  { return f.progress }
func (f *playStatusFamily) Badge() *status.Badge                  { return f.badge }
func (f *playStatusFamily) Light() *status.StatusLight            { return f.light }
func (f *playStatusFamily) Bar() *status.ProgressBar              { return f.bar }
func (f *playStatusFamily) Ring() *status.ProgressRing            { return f.ring }
func (f *playStatusFamily) Tick() *action.Button                  { return f.tick }
