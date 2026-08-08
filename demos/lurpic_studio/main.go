// Lurpic Studio — realtime interactable documentation for the lurpicUI
// framework. Spec: devdocs/plans/lurpic-studio-redesign.md.
package main

import (
	"embed"
	"fmt"
	"os"

	"codeburg.org/lexbit/lurpicui/app"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/studio"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/text"
)

//go:embed assets/NotoSans-Regular.ttf assets/NotoSans-Bold.ttf assets/metrics.csv
var embeddedFonts embed.FS

func main() {
	cfg := app.DefaultConfig("Lurpic Studio", 1280, 800)
	// Software backend for deterministic goldens and zero GPU-driver variance
	// (NFR-determinism).
	cfg.Render = app.RenderBackendSoftware
	fonts, err := loadEmbeddedFonts()
	if err != nil {
		fmt.Fprintf(os.Stderr, "lurpic_studio: %v\n", err)
		os.Exit(1)
	}
	cfg.Fonts = fonts
	seed, err := loadSeed()
	if err != nil {
		fmt.Fprintf(os.Stderr, "lurpic_studio: %v\n", err)
		os.Exit(1)
	}
	// F-diag-access + F-dirtysources: the E5 exhibit's dirty-snapshot observer
	// is installed as the app's runtime diagnostics hook; the exhibit reaches
	// the same object by pointer through the root builder.
	sink := studio.NewDirtySink(10)
	cfg.Diagnostics = sink
	if err := app.Run(cfg, func(ctx app.BuildContext) facet.FacetImpl {
		return studio.BuildRoot(ctx, sink, seed)
	}); err != nil {
		fmt.Fprintf(os.Stderr, "lurpic_studio: %v\n", err)
		os.Exit(1)
	}
}

// loadSeed reads and parses the metrics.csv snapshot (the seed for the shared
// AppState; the streaming feed reshapes it).
func loadSeed() ([]dataset.Row, error) {
	data, err := embeddedFonts.ReadFile("assets/metrics.csv")
	if err != nil {
		return nil, fmt.Errorf("read embedded metrics.csv: %w", err)
	}
	return dataset.Parse(data)
}

// loadEmbeddedFonts reads the vendored NotoSans faces. The repo's only other
// bundled copy lives in internal/fontdata and imports "testing", so it cannot
// be used by a non-test package.
func loadEmbeddedFonts() ([]text.FontSource, error) {
	regular, err := embeddedFonts.ReadFile("assets/NotoSans-Regular.ttf")
	if err != nil {
		return nil, fmt.Errorf("read embedded NotoSans-Regular: %w", err)
	}
	bold, err := embeddedFonts.ReadFile("assets/NotoSans-Bold.ttf")
	if err != nil {
		return nil, fmt.Errorf("read embedded NotoSans-Bold: %w", err)
	}
	return []text.FontSource{
		{Data: regular, Name: "Noto Sans Regular"},
		{Data: bold, Name: "Noto Sans Bold"},
	}, nil
}
