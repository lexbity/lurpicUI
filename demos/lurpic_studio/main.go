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
	if err := app.Run(cfg, buildRoot); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func buildRoot(ctx app.BuildContext) facet.FacetImpl {
	data, err := app.Asset("metrics.csv")
	if err == nil {
		rows, parseErr := dataset.Parse(data)
		if parseErr == nil {
			as := state.NewAppState(rows)
			return studio.BuildRoot(as, ctx)
		}
	}

	as := state.NewAppState(nil)
	return studio.BuildRoot(as, ctx)
}
