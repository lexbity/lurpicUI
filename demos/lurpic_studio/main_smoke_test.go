package main

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/app"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/studio"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/theme"
)

// TestMainSmoke_rootMeasuresToWindow drives the app root headlessly and asserts
// it measures/arranges to the requested window size. This proves the entry
// builder yields a facet the runtime can attach, measure, and arrange.
func TestMainSmoke_rootMeasuresToWindow(t *testing.T) {
	ctx := app.BuildContext{
		WindowSize:   gfx.Size{W: 1280, H: 800},
		ContentScale: 1,
		Theme:        theme.DefaultResolvedContext(),
	}
	seed, err := loadSeed()
	if err != nil {
		t.Fatalf("load seed: %v", err)
	}
	reg, err := studio.StudioLayerRegistry()
	if err != nil {
		t.Fatalf("layer registry: %v", err)
	}
	root := studio.BuildRoot(ctx, studio.NewDirtySink(5), seed, reg)
	if root == nil {
		t.Fatal("BuildRoot returned nil")
	}

	h := testkit.NewStandardHarness(t, 1280, 800, root)
	h.RunFrame()

	lr := root.Base().LayoutRole()
	if lr == nil {
		t.Fatal("root registers no layout role")
	}
	bounds := lr.ArrangedBounds
	if bounds.Width() != 1280 || bounds.Height() != 800 {
		t.Fatalf("root arranged bounds = %v, want 1280x800", bounds)
	}
}

// TestMainSmoke_fontsLoad verifies the vendored NotoSans faces embed and are
// valid font bytes (NFR-determinism: never load fonts from GOMODCACHE).
func TestMainSmoke_fontsLoad(t *testing.T) {
	fonts, err := loadEmbeddedFonts()
	if err != nil {
		t.Fatalf("loadEmbeddedFonts: %v", err)
	}
	if len(fonts) != 2 {
		t.Fatalf("loadEmbeddedFonts returned %d sources, want 2", len(fonts))
	}
	for _, src := range fonts {
		if len(src.Data) == 0 {
			t.Fatalf("font %q embeds no data", src.Name)
		}
		if src.Name == "" {
			t.Fatal("font source has empty name")
		}
	}
}
