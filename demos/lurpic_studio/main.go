package main

import (
	"fmt"
	"os"

	"codeburg.org/lexbit/lurpicui/app"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/studio"
	"codeburg.org/lexbit/lurpicui/facet"
)

func main() {
	cfg := app.DefaultConfig("Lurpic Studio", 1280, 800)
	cfg.Render = app.RenderBackendSoftware

	raw, err := os.ReadFile("demos/lurpic_studio/assets/metrics.csv")
	if err != nil {
		raw, err = app.Asset("metrics.csv")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading metrics.csv: %v\n", err)
			os.Exit(1)
		}
	}
	rows, err := dataset.Parse(raw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing metrics.csv: %v\n", err)
		os.Exit(1)
	}
	appState := state.NewAppState(rows)

	if err := app.Run(cfg, func(ctx app.BuildContext) facet.FacetImpl {
		return studio.NewRoot(appState, ctx.WindowSize, ctx.FontRegistry)
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
