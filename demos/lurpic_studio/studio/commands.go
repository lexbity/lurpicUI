package studio

import (
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/runtime"
)

func registerCommands(reg *runtime.CommandRegistry, as *state.AppState) {
	chartTypes := []state.ChartType{state.ChartLine, state.ChartBar, state.ChartArea, state.ChartScatter}
	for _, ct := range chartTypes {
		reg.Register(runtime.CommandEntry{
			ID:       "chart-" + string(ct),
			Title:    "Switch to " + string(ct) + " chart",
			Category: "Chart",
			Execute: func() {
				as.ChartType.Set(ct)
			},
		})
	}

	reg.Register(runtime.CommandEntry{
		ID:       "select-na",
		Title:    "Select NA source",
		Category: labelSources,
		Execute: func() {
			as.SelectedSource.Set("NA")
		},
	})
	reg.Register(runtime.CommandEntry{
		ID:       "select-eu",
		Title:    "Select EU source",
		Category: labelSources,
		Execute: func() {
			as.SelectedSource.Set("EU")
		},
	})
	reg.Register(runtime.CommandEntry{
		ID:       "select-apac",
		Title:    "Select APAC source",
		Category: labelSources,
		Execute: func() {
			as.SelectedSource.Set("APAC")
		},
	})
	reg.Register(runtime.CommandEntry{
		ID:       "select-latam",
		Title:    "Select LATAM source",
		Category: labelSources,
		Execute: func() {
			as.SelectedSource.Set(labelLATAM)
		},
	})

	reg.Register(runtime.CommandEntry{
		ID:       "reset-all",
		Title:    "Reset all filters",
		Category: "Data",
		Execute: func() {
			as.SelectedSource.Set("")
			as.Page.Set(0)
			as.Aggregation.Set(state.AggNone)
		},
	})
	reg.Register(runtime.CommandEntry{
		ID:       "show-grid",
		Title:    "Toggle grid",
		Category: "View",
		Execute: func() {
			as.ShowGrid.Set(!as.ShowGrid.Get())
		},
	})
}
