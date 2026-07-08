package studio

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/marks/status"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
)

type statusBar struct {
	row          *layout.RowLayout
	light        *status.StatusLight
	progressBar  *status.ProgressBar
	progressRing *status.ProgressRing
	badge        *status.Badge
	statusText   *primitive.Text
}

func newStatusBar(as *state.AppState) *statusBar {
	sb := &statusBar{}

	sb.light = status.NewStatusLight("Connection")
	sb.light.ShowLabel = marks.Const(false)

	progressF32 := store.NewValueStore[float32](0)
	as.JobProgress.OnChange.Subscribe(func(c signal.Change[float64]) {
		progressF32.Set(float32(c.New))
	})

	sb.progressBar = status.NewProgressBar("Job progress")
	sb.progressBar.Value = marks.FromStore[float32](progressF32, 0)

	sb.progressRing = status.NewProgressRing("Job progress")
	sb.progressRing.Value = marks.FromStore[float32](progressF32, 0)

	sb.badge = status.NewBadge("")
	allowLinear(sb.badge)

	sb.statusText = primitive.NewText(marks.Const("Ready"))

	sb.row = layout.NewRowLayout()
	sb.row.Gap = 0
	sb.row.Add(layout.Fixed(sb.light))
	sb.row.Add(layout.Fixed(sb.progressBar))
	sb.row.Add(layout.Fixed(sb.progressRing))
	sb.row.Add(layout.Fixed(sb.badge))
	sb.row.Add(layout.Flexible(sb.statusText, 1))

	wireStatusLight(sb.light, as)
	wireBadge(sb.badge, as)
	wireStatusText(sb.statusText, as)

	return sb
}

func wireStatusLight(light *status.StatusLight, as *state.AppState) {
	as.Connection.OnChange.Subscribe(func(c signal.Change[state.ConnState]) {
		light.Label = marks.Const(string(c.New))
		light.Disabled = marks.Const(c.New == state.ConnDisconnected || c.New == state.ConnError)
	})
	light.Label = marks.Const(string(as.Connection.Get()))
}

func wireBadge(badge *status.Badge, as *state.AppState) {
	updateBadge := func() {
		source := as.SelectedSource.Get()
		count := countRowsForSource(as, source)
		if count > 0 {
			badge.Label = marks.Const(fmt.Sprintf("%d rows", count))
		} else {
			badge.Label = marks.Const("")
		}
	}
	updateBadge()
	as.SelectedSource.OnChange.Subscribe(func(c signal.Change[string]) {
		updateBadge()
	})
}

func wireStatusText(text *primitive.Text, as *state.AppState) {
	as.Connection.OnChange.Subscribe(func(c signal.Change[state.ConnState]) {
		switch c.New {
		case state.ConnConnecting:
			text.Content = marks.Const("Connecting...")
		case state.ConnConnected:
			text.Content = marks.Const("Ready")
		case state.ConnDisconnected:
			text.Content = marks.Const("Disconnected")
		case state.ConnError:
			text.Content = marks.Const("Error")
		}
	})
}
