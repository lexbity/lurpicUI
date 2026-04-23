package shell

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/ui_diagnostic_scene/scene"
)

func TestNewRootFacet(t *testing.T) {
	shaper := &text.Shaper{}
	registry := scene.NewRegistry()
	root := NewRootFacet(theme.Default(), shaper, registry)

	if root == nil {
		t.Fatal("expected non-nil root facet")
	}
	if root.Base() == nil {
		t.Fatal("expected non-nil base facet")
	}
}

func TestRootFacet_SelectedSceneID_initially_empty(t *testing.T) {
	shaper := &text.Shaper{}
	registry := scene.NewRegistry()
	root := NewRootFacet(theme.Default(), shaper, registry)

	id := root.SelectedSceneID().Get()
	if id != "" {
		t.Fatalf("expected empty initial scene ID, got %s", id)
	}
}

func TestRootFacet_Registry(t *testing.T) {
	shaper := &text.Shaper{}
	registry := scene.NewRegistry()
	root := NewRootFacet(theme.Default(), shaper, registry)

	if root.Registry() != registry {
		t.Fatal("expected registry to match")
	}
}

func TestLogsPanelFacet_empty_does_not_crash(t *testing.T) {
	panel := NewLogsPanelFacet(theme.Default(), &text.Shaper{})
	if panel == nil {
		t.Fatal("expected non-nil panel")
	}

	// AppendLog with empty state should not crash
	panel.AppendLog(LogEntry{Category: "test", Message: "test message"})

	// Clear should not crash
	panel.Clear()

	// MaxEntries operations should not crash
	panel.SetMaxEntries(50)
	if panel.maxEntries != 50 {
		t.Fatalf("expected maxEntries 50, got %d", panel.maxEntries)
	}
}

func TestDiagnosticsPanelFacet_empty_does_not_crash(t *testing.T) {
	panel := NewDiagnosticsPanelFacet(theme.Default(), &text.Shaper{})
	if panel == nil {
		t.Fatal("expected non-nil panel")
	}

	// Toggle operations should not crash
	panel.SetOverlayEnabled(true)
	panel.SetShowBounds(true)
	panel.SetShowHitRegions(true)
	panel.SetShowFocus(true)

	// Verify state
	if !panel.overlayEnabled {
		t.Fatal("expected overlayEnabled to be true")
	}
}

func TestSceneHostFacet_Reset_uninitialized(t *testing.T) {
	host := NewSceneHostFacet(theme.Default(), &text.Shaper{})
	if host == nil {
		t.Fatal("expected non-nil host")
	}

	// Reset on uninitialized scene should not crash
	host.Reset()

	if host.CurrentSceneID() != "" {
		t.Fatal("expected empty scene ID after reset")
	}
}

func TestSceneHostFacet_LoadScene_empty_registry(t *testing.T) {
	host := NewSceneHostFacet(theme.Default(), &text.Shaper{})
	registry := scene.NewRegistry()

	// Load scene from empty registry should fail safely
	host.LoadScene("nonexistent", registry)

	if host.CurrentScene() != nil {
		t.Fatal("expected nil scene for nonexistent ID")
	}
}

func TestSceneNavFacet_SelectScene(t *testing.T) {
	registry := scene.NewRegistry()
	registry.Register(scene.Definition{
		ID:          "test",
		DisplayName: "Test",
		Factory:     func() scene.Scene { return nil },
	})

	nav := NewSceneNavFacet(theme.Default(), &text.Shaper{}, registry)
	if nav == nil {
		t.Fatal("expected non-nil nav")
	}

	// Select scene
	selected := ""
	nav.OnSceneSelected.Subscribe(func(id string) {
		selected = id
	})

	nav.SelectScene("test")

	if selected != "test" {
		t.Fatalf("expected signal with 'test', got %s", selected)
	}
	if nav.SelectedScene() != "test" {
		t.Fatalf("expected SelectedScene 'test', got %s", nav.SelectedScene())
	}

	// Selecting same ID should not re-emit
	selected = ""
	nav.SelectScene("test")
	if selected != "" {
		t.Fatal("expected no signal on re-selection of same ID")
	}
}

func TestTopBarFacet_SetInfo(t *testing.T) {
	bar := NewTopBarFacet(theme.Default(), &text.Shaper{})
	if bar == nil {
		t.Fatal("expected non-nil bar")
	}

	// Should not crash
	bar.SetInfo("Dark", "Vulkan", "Windows")

	if bar.text == "" {
		t.Fatal("expected text to be set")
	}
}
