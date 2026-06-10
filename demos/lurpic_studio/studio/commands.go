package studio

import (
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/runtime"
)

func registerCommands(registry *runtime.CommandRegistry, appState *state.AppState) {
	registry.Register(runtime.CommandEntry{
		ID:       "chart_line",
		Title:    "Switch to Line Chart",
		Category: "Chart",
		Keywords: []string{"line", "chart", "switch"},
		Execute:  func() { appState.ChartType.Set(state.ChartLine) },
	})
	registry.Register(runtime.CommandEntry{
		ID:       "chart_bar",
		Title:    "Switch to Bar Chart",
		Category: "Chart",
		Keywords: []string{"bar", "chart", "switch"},
		Execute:  func() { appState.ChartType.Set(state.ChartBar) },
	})
	registry.Register(runtime.CommandEntry{
		ID:       "chart_area",
		Title:    "Switch to Area Chart",
		Category: "Chart",
		Keywords: []string{"area", "chart", "switch"},
		Execute:  func() { appState.ChartType.Set(state.ChartArea) },
	})
	registry.Register(runtime.CommandEntry{
		ID:       "chart_scatter",
		Title:    "Switch to Scatter Chart",
		Category: "Chart",
		Keywords: []string{"scatter", "chart", "switch"},
		Execute:  func() { appState.ChartType.Set(state.ChartPoint) },
	})
	registry.Register(runtime.CommandEntry{
		ID:       "show_grid",
		Title:    "Toggle Grid",
		Category: "Display",
		Keywords: []string{"grid", "toggle", "show"},
		Execute:  func() { appState.ShowGrid.Set(!appState.ShowGrid.Get()) },
	})
	registry.Register(runtime.CommandEntry{
		ID:       "opacity_high",
		Title:    "Set Opacity to 90%",
		Category: "Chart",
		Keywords: []string{"opacity", "fill", "alpha"},
		Execute:  func() { appState.Opacity.Set(0.9) },
	})
	registry.Register(runtime.CommandEntry{
		ID:       "reset_view",
		Title:    "Reset View",
		Category: "Display",
		Keywords: []string{"reset", "view", "default"},
		Execute:  func() { appState.Page.Set(1); appState.SelectedSource.Set("") },
	})
}
