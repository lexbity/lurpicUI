package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"codeburg.org/lexbit/lurpicui/app"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/ui_replay/model"
	"codeburg.org/lexbit/ui_replay/store"
	"codeburg.org/lexbit/ui_replay/ui"
)

func main() {
	var (
		windowWidth   = flag.Int("width", 1400, "window width")
		windowHeight  = flag.Int("height", 900, "window height")
		scenarioDir   = flag.String("scenario-dir", defaultScenarioDir(), "directory containing scenario files")
		historyDir    = flag.String("history-dir", defaultHistoryDir(), "directory for replay history")
		exportDir     = flag.String("export-dir", defaultExportDir(), "directory for artifact exports")
	)
	flag.Parse()

	config := app.DefaultConfig("UI Replay", *windowWidth, *windowHeight)
	config.Fonts = defaultFontSources()
	config.Theme = theme.Default()

	meta := model.DefaultBuildMetadata()

	if err := store.InitRegistry(*scenarioDir, *historyDir, *exportDir); err != nil {
		log.Printf("Warning: failed to initialize registry: %v", err)
	}

	if err := app.Run(config, func(ctx app.BuildContext) facet.FacetImpl {
		shaper := text.NewShaper(ctx.FontRegistry)
		shaper.SetContentScale(ctx.ContentScale)
		return ui.NewReplayRootFacet(ctx.Theme, shaper, meta)
	}); err != nil {
		log.Fatal(err)
	}
}

func defaultFontSources() []app.FontSource {
	candidates := []string{
		"/usr/share/fonts/noto/NotoSans-Regular.ttf",
		"/usr/share/fonts/TTF/DejaVuSans.ttf",
		"/usr/share/fonts/liberation/LiberationSans-Regular.ttf",
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return []app.FontSource{{Path: path, Name: "Noto Sans"}}
		}
	}
	return nil
}

func defaultScenarioDir() string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), "testdata", "scenarios")
	}
	return "./testdata/scenarios"
}

func defaultHistoryDir() string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), "testdata", "history")
	}
	return "./testdata/history"
}

func defaultExportDir() string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), "export")
	}
	return "./export"
}
