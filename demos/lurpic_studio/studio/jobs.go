package studio

import (
	"time"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
)

func simulateReloadJob(as *state.AppState) {
	as.Connection.Set(state.ConnConnecting)
	as.JobProgress.Set(float64(0))

	steps := 20
	stepDur := 50 * time.Millisecond

	for i := 1; i <= steps; i++ {
		time.Sleep(stepDur)
		progress := float64(i) / float64(steps)
		as.JobProgress.Set(progress)
	}

	as.JobProgress.Set(float64(0))
	as.Connection.Set(state.ConnConnected)

	// Simulate new data: replace rows with same dataset (timestamp update)
	rows := as.Rows.All()
	as.Rows.Replace(rows)
}
