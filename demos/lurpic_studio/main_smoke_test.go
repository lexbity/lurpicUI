package main

import (
	"os"
	"path/filepath"
	"testing"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/studio"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
)

func TestEndToEndBuild(t *testing.T) {
	raw, err := os.ReadFile(filepath.FromSlash("assets/metrics.csv"))
	if err != nil {
		t.Fatalf("reading metrics.csv: %v", err)
	}
	rows, err := dataset.Parse(raw)
	if err != nil {
		t.Fatalf("parsing metrics.csv: %v", err)
	}
	if len(rows) != 40 {
		t.Fatalf("expected 40 rows, got %d", len(rows))
	}
	appState := state.NewAppState(rows)
	fonts := testkit.TestFontRegistry(t)

	root := studio.NewRoot(appState, gfx.Size{W: 1280, H: 800}, fonts)
	if root == nil {
		t.Fatal("NewRoot returned nil")
	}
	base := root.Base()
	if base == nil {
		t.Fatal("root.Base() returned nil")
	}
	if base.LayoutRole() == nil {
		t.Fatal("root has no LayoutRole")
	}
	if base.RenderRole() == nil {
		t.Fatal("root has no RenderRole")
	}
}

func TestEndToEndBuildNarrow(t *testing.T) {
	raw, err := os.ReadFile(filepath.FromSlash("assets/metrics.csv"))
	if err != nil {
		t.Fatalf("reading metrics.csv: %v", err)
	}
	rows, err := dataset.Parse(raw)
	if err != nil {
		t.Fatalf("parsing metrics.csv: %v", err)
	}
	appState := state.NewAppState(rows)
	fonts := testkit.TestFontRegistry(t)

	root := studio.NewRoot(appState, gfx.Size{W: 480, H: 800}, fonts)
	if root == nil {
		t.Fatal("NewRoot returned nil for narrow mode")
	}
	base := root.Base()
	layoutRole := base.LayoutRole()
	result := layoutRole.Measure(facet.MeasureContext{}, facet.Constraints{MaxSize: gfx.Size{W: 480, H: 800}})
	if result.Size.W != 480 || result.Size.H != 800 {
		t.Errorf("expected 480x800, got %v", result.Size)
	}
	if appState.LayoutMode.Get() != state.LayoutNarrow {
		t.Errorf("expected LayoutNarrow after measure for 480px, got %v", appState.LayoutMode.Get())
	}
}

func TestMainWithAssetPath(t *testing.T) {
	raw, err := os.ReadFile(filepath.FromSlash("assets/metrics.csv"))
	if err != nil {
		t.Fatalf("reading metrics.csv: %v", err)
	}
	rows, err := dataset.Parse(raw)
	if err != nil {
		t.Fatalf("parsing metrics.csv: %v", err)
	}
	if len(rows) != 40 {
		t.Fatalf("expected 40 rows, got %d", len(rows))
	}
}
