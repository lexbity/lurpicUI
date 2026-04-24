package main

import (
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/ui_diagnostic_scene/scene"
)

func TestRegisterScenes_CoversRequiredTaxonomy(t *testing.T) {
	registry := scene.NewRegistry()
	registerScenes(registry)

	wantIDs := []string{
		"catalog-lite",
		"interaction",
		"layout",
		"input-focus",
		"stress",
		"projection",
		"animation",
		"theme",
		"store-signal",
		"text-ime",
		"annotation",
		"chart",
		"uinav",
		"uinotification",
	}
	for _, id := range wantIDs {
		def, ok := registry.Get(id)
		if !ok {
			t.Fatalf("expected scene %q to be registered", id)
		}
		if def.Factory == nil {
			t.Fatalf("expected scene %q to have a factory", id)
		}
		scene := def.Factory()
		if scene == nil {
			t.Fatalf("expected scene %q factory to produce a scene", id)
		}
		if scene.SceneID() == "" {
			t.Fatalf("expected scene %q to report a stable scene ID", id)
		}
		if scene.DisplayName() == "" {
			t.Fatalf("expected scene %q to report a display name", id)
		}
		if scene.BuildRoot() == nil {
			t.Fatalf("expected scene %q to build a root facet", id)
		}
		if got := scene.ExportState(); got == nil {
			t.Fatalf("expected scene %q to export state", id)
		}
	}
}

func TestRegisterScenes_HasRepresentativeDescriptions(t *testing.T) {
	registry := scene.NewRegistry()
	registerScenes(registry)

	wantContains := map[string]string{
		"catalog-lite":   "nested layout",
		"interaction":    "disabled",
		"layout":         "overflow",
		"input-focus":    "disabled focus targets",
		"projection":     "anchor forwarding",
		"annotation":     "callouts",
		"chart":          "density-aware",
		"uinotification": "progress",
	}
	for id, snippet := range wantContains {
		def, ok := registry.Get(id)
		if !ok {
			t.Fatalf("expected scene %q to be registered", id)
		}
		if def.Description == "" {
			t.Fatalf("expected scene %q to have a description", id)
		}
		if !containsString(def.Description, snippet) {
			t.Fatalf("expected scene %q description to mention %q, got %q", id, snippet, def.Description)
		}
	}
}

func TestRegisterScenes_CoversRequiredFamilies(t *testing.T) {
	registry := scene.NewRegistry()
	registerScenes(registry)

	families := map[string]bool{}
	for _, def := range registry.GetAll() {
		for _, family := range def.Families {
			families[family] = true
		}
	}

	for _, family := range []string{"basic", "structure", "uiinput", "annotation", "chart", "uinav", "uinotification"} {
		if !families[family] {
			t.Fatalf("expected family %q to be covered by the registry", family)
		}
	}
}

func TestRegisterScenes_CoverageMapIsStable(t *testing.T) {
	registry := scene.NewRegistry()
	registerScenes(registry)

	got := map[string][]string{}
	for _, def := range registry.GetAll() {
		got[def.ID] = append([]string(nil), def.Families...)
	}

	want := map[string][]string{
		"catalog-lite":   {"basic", "structure"},
		"interaction":    {"uiinput"},
		"layout":         {"structure"},
		"input-focus":    {"uiinput"},
		"stress":         {"basic", "structure", "uiinput"},
		"projection":     {"structure"},
		"animation":      {"basic"},
		"theme":          {"basic"},
		"store-signal":   {"basic"},
		"text-ime":       {"uiinput"},
		"annotation":     {"annotation"},
		"chart":          {"chart"},
		"uinav":          {"uinav"},
		"uinotification": {"uinotification"},
	}
	for id, wantFamilies := range want {
		families, ok := got[id]
		if !ok {
			t.Fatalf("expected scene %q to be registered", id)
		}
		if len(families) != len(wantFamilies) {
			t.Fatalf("scene %q families mismatch: got %v want %v", id, families, wantFamilies)
		}
		for i := range wantFamilies {
			if families[i] != wantFamilies[i] {
				t.Fatalf("scene %q families mismatch: got %v want %v", id, families, wantFamilies)
			}
		}
	}
}

func TestRegisterScenes_BuildsWithDefaultTheme(t *testing.T) {
	registry := scene.NewRegistry()
	registerScenes(registry)

	for _, id := range []string{"theme", "store-signal", "text-ime", "annotation", "chart", "uinav", "uinotification"} {
		def, _ := registry.Get(id)
		scene := def.Factory()
		if scene == nil {
			t.Fatalf("expected scene %q", id)
		}
		if scene.BuildRoot() == nil {
			t.Fatalf("expected scene %q to build with a default theme", id)
		}
		scene.ApplyTheme(theme.Default())
		scene.ApplyDensity(1)
		_ = scene.ExportState()
	}
}

func TestDefaultFontSources_doesNotPanic(t *testing.T) {
	_ = defaultFontSources()
	_ = text.Shaper{}
}

func containsString(s, substr string) bool {
	return strings.Contains(s, substr)
}
