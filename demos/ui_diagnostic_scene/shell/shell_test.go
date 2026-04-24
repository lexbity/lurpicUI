package shell

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	bundle "codeburg.org/lexbit/ui_diagnostic_scene/export"
	"codeburg.org/lexbit/ui_diagnostic_scene/scene"
	"codeburg.org/lexbit/ui_diagnostic_scene/scenes"
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
	panel := NewDiagnosticsPanelFacet(theme.Default(), &text.Shaper{}, nil)
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

func TestTopBarFacet_CommandButtonsEmitSignals(t *testing.T) {
	bar := NewTopBarFacet(theme.Default(), &text.Shaper{})
	if bar == nil {
		t.Fatal("expected non-nil bar")
	}

	bar.layout.OnArrange(gfx.RectFromXYWH(0, 0, 900, 32))

	cases := []struct {
		id      string
		emitted *bool
	}{
		{id: "reset", emitted: new(bool)},
		{id: "theme", emitted: new(bool)},
		{id: "density", emitted: new(bool)},
		{id: "bounds", emitted: new(bool)},
		{id: "hit", emitted: new(bool)},
		{id: "focus", emitted: new(bool)},
		{id: "stress", emitted: new(bool)},
	}

	bar.OnReset.Subscribe(func(_ struct{}) { *cases[0].emitted = true })
	bar.OnThemeNext.Subscribe(func(_ struct{}) { *cases[1].emitted = true })
	bar.OnDensityNext.Subscribe(func(_ struct{}) { *cases[2].emitted = true })
	bar.OnToggleBounds.Subscribe(func(_ struct{}) { *cases[3].emitted = true })
	bar.OnToggleHit.Subscribe(func(_ struct{}) { *cases[4].emitted = true })
	bar.OnToggleFocus.Subscribe(func(_ struct{}) { *cases[5].emitted = true })
	bar.OnToggleStress.Subscribe(func(_ struct{}) { *cases[6].emitted = true })

	for _, tc := range cases {
		rect, ok := bar.buttonRects[tc.id]
		if !ok || rect.IsEmpty() {
			t.Fatalf("expected layout rect for %s", tc.id)
		}
		center := gfx.Point{X: (rect.Min.X + rect.Max.X) / 2, Y: (rect.Min.Y + rect.Max.Y) / 2}
		if !bar.input.OnPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: center, Button: platform.PointerLeft}) {
			t.Fatalf("expected press on %s to be handled", tc.id)
		}
		if !bar.input.OnPointer(facet.PointerEvent{Kind: platform.PointerRelease, Position: center, Button: platform.PointerLeft}) {
			t.Fatalf("expected release on %s to be handled", tc.id)
		}
		if !*tc.emitted {
			t.Fatalf("expected %s button to emit", tc.id)
		}
	}
}

func TestRootFacet_TopBarCommandsMutateState(t *testing.T) {
	registry := scene.NewRegistry()
	registry.Register(scene.Definition{
		ID:          "alpha",
		DisplayName: "Alpha",
		Factory:     func() scene.Scene { return newCountingScene("alpha") },
	})

	root := NewRootFacet(theme.Default(), &text.Shaper{}, registry)
	facet.Attach(root, facet.AttachContext{})
	facet.Activate(root)

	if got := root.SelectedSceneID().Get(); got != "alpha" {
		t.Fatalf("expected alpha selected, got %q", got)
	}

	triggerTopBarButton(t, root.topBar, "theme")
	if root.themeMode != ThemeModeNight {
		t.Fatalf("expected night theme after cycle, got %s", root.themeMode.String())
	}
	if root.SelectedSceneID().Get() != "alpha" {
		t.Fatal("expected theme change to preserve selection")
	}
	if root.topBar == nil || root.topBar.text == "" {
		t.Fatal("expected top bar status text")
	}

	triggerTopBarButton(t, root.topBar, "density")
	if root.densityMode != DensityModeComfortable {
		t.Fatalf("expected comfortable density after cycle, got %s", root.densityMode.String())
	}
	if root.SelectedSceneID().Get() != "alpha" {
		t.Fatal("expected density change to preserve selection")
	}
}

func TestSceneHostFacet_ApplyThemeAndDensity_propagate(t *testing.T) {
	registry := scene.NewRegistry()
	selected := newCountingScene("alpha")
	registry.Register(scene.Definition{
		ID:          "alpha",
		DisplayName: "Alpha",
		Factory:     func() scene.Scene { return selected },
	})

	root := NewRootFacet(theme.Default(), &text.Shaper{}, registry)
	facet.Attach(root, facet.AttachContext{})
	facet.Activate(root)

	if selected.themeCount == 0 {
		t.Fatal("expected theme application on initial mount")
	}
	if selected.densityCount == 0 {
		t.Fatal("expected density application on initial mount")
	}

	root.sceneHost.ApplyTheme(theme.Default())
	root.sceneHost.ApplyDensity(1.15)

	if selected.themeCount < 2 {
		t.Fatalf("expected theme reapplication, got %d", selected.themeCount)
	}
	if selected.densityCount < 2 {
		t.Fatalf("expected density reapplication, got %d", selected.densityCount)
	}
}

func TestRootFacet_TopBarStressToggleRebuildsScene(t *testing.T) {
	registry := scene.NewRegistry()
	registry.Register(scene.Definition{
		ID:          "stress",
		DisplayName: "Stress",
		Factory:     func() scene.Scene { return scenes.NewStressScene() },
	})

	root := NewRootFacet(theme.Default(), &text.Shaper{}, registry)
	facet.Attach(root, facet.AttachContext{})
	facet.Activate(root)

	oldRoot := root.sceneHost.MountedRoot()
	if oldRoot == nil {
		t.Fatal("expected initial mounted root")
	}

	triggerTopBarButton(t, root.topBar, "stress")

	if !root.stressMode {
		t.Fatal("expected stress mode to enable")
	}
	if root.sceneHost.MountedRoot() == nil {
		t.Fatal("expected mounted root after stress toggle")
	}
	if root.sceneHost.MountedRoot() == oldRoot {
		t.Fatal("expected stress toggle to rebuild the mounted root")
	}
}

func TestRootFacet_Attach_mounts_default_scene(t *testing.T) {
	registry := scene.NewRegistry()
	first := newCountingScene("alpha")
	second := newCountingScene("beta")

	registry.Register(scene.Definition{
		ID:          "alpha",
		DisplayName: "Alpha",
		Factory:     func() scene.Scene { return first },
	})
	registry.Register(scene.Definition{
		ID:          "beta",
		DisplayName: "Beta",
		Factory:     func() scene.Scene { return second },
	})

	root := NewRootFacet(theme.Default(), &text.Shaper{}, registry)
	facet.Attach(root, facet.AttachContext{})

	if got := root.SelectedSceneID().Get(); got != "" {
		t.Fatalf("expected no selection before activation, got %q", got)
	}

	facet.Activate(root)

	if got := root.SelectedSceneID().Get(); got != "alpha" {
		t.Fatalf("expected default selection alpha, got %q", got)
	}
	if got := root.sceneHost.CurrentSceneID(); got != "alpha" {
		t.Fatalf("expected mounted scene alpha, got %q", got)
	}
	if root.sceneHost.CurrentScene() != first {
		t.Fatal("expected current scene to match first factory result")
	}
	if root.sceneHost.MountedRoot() == nil {
		t.Fatal("expected mounted scene root")
	}
	if root.sceneHost.MountedRoot() != first.root {
		t.Fatal("expected mounted root to come from the selected scene")
	}
	if got := first.root.Base().State(); got != facet.StateActive {
		t.Fatalf("expected mounted root active after root activation, got %s", got)
	}
}

func TestSceneHostFacet_Reset_rebuilds_mounted_root(t *testing.T) {
	registry := scene.NewRegistry()
	selected := newCountingScene("alpha")
	registry.Register(scene.Definition{
		ID:          "alpha",
		DisplayName: "Alpha",
		Factory:     func() scene.Scene { return selected },
	})

	root := NewRootFacet(theme.Default(), &text.Shaper{}, registry)
	facet.Attach(root, facet.AttachContext{})
	facet.Activate(root)

	oldRoot := selected.root
	if oldRoot == nil {
		t.Fatal("expected initial mounted root")
	}

	root.sceneHost.Reset()

	if selected.resetCount != 1 {
		t.Fatalf("expected one reset, got %d", selected.resetCount)
	}
	if selected.buildCount != 2 {
		t.Fatalf("expected rebuild after reset, got %d builds", selected.buildCount)
	}
	if root.sceneHost.MountedRoot() == nil {
		t.Fatal("expected remounted root after reset")
	}
	if root.sceneHost.MountedRoot() == oldRoot {
		t.Fatal("expected reset to rebuild a fresh root")
	}
	if got := oldRoot.Base().State(); got != facet.StateDisposed {
		t.Fatalf("expected old root disposed, got %s", got)
	}
	if got := root.sceneHost.MountedRoot().Base().State(); got != facet.StateActive {
		t.Fatalf("expected remounted root active, got %s", got)
	}
}

func TestSceneHostFacet_LoadScene_missing_clears_mounted_scene(t *testing.T) {
	registry := scene.NewRegistry()
	selected := newCountingScene("alpha")
	registry.Register(scene.Definition{
		ID:          "alpha",
		DisplayName: "Alpha",
		Factory:     func() scene.Scene { return selected },
	})

	root := NewRootFacet(theme.Default(), &text.Shaper{}, registry)
	facet.Attach(root, facet.AttachContext{})
	facet.Activate(root)

	root.sceneHost.LoadScene("missing", registry)

	if root.sceneHost.CurrentScene() != nil {
		t.Fatal("expected current scene cleared for missing ID")
	}
	if got := root.sceneHost.CurrentSceneID(); got != "" {
		t.Fatalf("expected current scene ID cleared, got %q", got)
	}
	if root.sceneHost.MountedRoot() != nil {
		t.Fatal("expected mounted root cleared for missing ID")
	}
	if got := len(root.sceneHost.Children()); got != 0 {
		t.Fatalf("expected host child list cleared, got %d children", got)
	}
}

func TestRootFacet_ExportBundle(t *testing.T) {
	registry := scene.NewRegistry()
	registry.Register(scene.Definition{
		ID:          "alpha",
		DisplayName: "Alpha",
		Description: "Alpha scene",
		Families:    []string{"basic"},
		Factory:     func() scene.Scene { return newCountingScene("alpha") },
	})

	root := NewRootFacet(theme.Default(), &text.Shaper{}, registry)
	facet.Attach(root, facet.AttachContext{})
	facet.Activate(root)
	root.leftNav.SelectScene("alpha")
	root.log("scene load", "Manual export test")

	got := root.ExportBundle("run-123")
	if got.Manifest.RunID != "run-123" {
		t.Fatalf("expected run id run-123, got %q", got.Manifest.RunID)
	}
	if got.Manifest.SceneID != "alpha" {
		t.Fatalf("expected manifest scene id alpha, got %q", got.Manifest.SceneID)
	}
	if got.Scene.SceneID != "alpha" {
		t.Fatalf("expected scene snapshot alpha, got %q", got.Scene.SceneID)
	}
	if len(got.Logs) == 0 {
		t.Fatal("expected bundle logs")
	}
	if got.Logs[0].Ordinal == 0 {
		t.Fatal("expected bundle log ordinals to be preserved")
	}
	if len(got.Artifacts) == 0 {
		t.Fatal("expected bundle artifacts")
	}
	wantArtifact := bundle.ArtifactName("bundle-manifest", got.Manifest, "json")
	if got.Artifacts[0].Name != wantArtifact {
		t.Fatalf("expected first artifact name %q, got %q", wantArtifact, got.Artifacts[0].Name)
	}
}

func TestRootFacet_RepeatedSceneSwitchesStayBounded(t *testing.T) {
	registry := scene.NewRegistry()
	registry.Register(scene.Definition{
		ID:          "alpha",
		DisplayName: "Alpha",
		Factory:     func() scene.Scene { return newCountingScene("alpha") },
	})
	registry.Register(scene.Definition{
		ID:          "beta",
		DisplayName: "Beta",
		Factory:     func() scene.Scene { return newCountingScene("beta") },
	})

	root := NewRootFacet(theme.Default(), &text.Shaper{}, registry)
	facet.Attach(root, facet.AttachContext{})
	facet.Activate(root)

	for i := 0; i < 8; i++ {
		want := "alpha"
		if i%2 == 1 {
			want = "beta"
		}
		root.leftNav.SelectScene(want)
		if got := root.sceneHost.CurrentSceneID(); got != want {
			t.Fatalf("expected current scene %q, got %q", want, got)
		}
		if root.sceneHost.MountedRoot() == nil {
			t.Fatalf("expected mounted root for %q", want)
		}
		if got := len(root.sceneHost.Children()); got != 1 {
			t.Fatalf("expected one mounted child after switch, got %d", got)
		}
	}

	if got := len(root.logs); got > root.maxLogs {
		t.Fatalf("expected bounded root logs, got %d > %d", got, root.maxLogs)
	}
}

func TestStressScene_RecordsFailureSources(t *testing.T) {
	s := scenes.NewStressScene()

	s.TriggerThemeChurn(nil)
	s.TriggerDensityChurn(nil)
	s.TriggerInputSpam(3)
	s.TriggerSceneResetChurn(2)
	s.TriggerResizeChurn(4)
	s.TriggerMountUnmountChurn(1)

	report := s.GetStressReport()
	if report["failure_source"] == "" {
		t.Fatal("expected failure source to be recorded")
	}
	failures, ok := report["failure_counts"].(map[string]int)
	if !ok {
		t.Fatalf("expected failure_counts map, got %T", report["failure_counts"])
	}
	if failures["theme"] == 0 {
		t.Fatal("expected theme failure to be recorded")
	}
	if len(report["recent_notes"].([]string)) == 0 {
		t.Fatal("expected recent notes to be captured")
	}
}

type countingScene struct {
	id           string
	root         *countingFacet
	buildCount   int
	resetCount   int
	themeCount   int
	densityCount int
}

func newCountingScene(id string) *countingScene {
	return &countingScene{id: id}
}

func (s *countingScene) SceneID() string { return s.id }
func (s *countingScene) DisplayName() string {
	return s.id
}
func (s *countingScene) BuildRoot() facet.FacetImpl {
	s.buildCount++
	if s.root == nil {
		s.root = newCountingFacet()
	}
	return s.root
}
func (s *countingScene) Reset() {
	s.resetCount++
	s.root = nil
}
func (s *countingScene) ApplyTheme(theme.Context)          { s.themeCount++ }
func (s *countingScene) ApplyDensity(float32)              { s.densityCount++ }
func (s *countingScene) Capabilities() scene.CapabilitySet { return scene.CapabilitySet{} }
func (s *countingScene) ExportState() map[string]any       { return nil }
func (s *countingScene) ImportState(map[string]any)        {}

type countingFacet struct {
	facet.Facet
	attachCount   int
	activateCount int
	detachCount   int
}

func newCountingFacet() *countingFacet {
	return &countingFacet{Facet: facet.NewFacet()}
}

func (f *countingFacet) Base() *facet.Facet {
	f.Facet.BindImpl(f)
	return &f.Facet
}
func (f *countingFacet) OnAttach(facet.AttachContext) {
	f.attachCount++
}
func (f *countingFacet) OnDetach() {
	f.detachCount++
}
func (f *countingFacet) OnActivate() {
	f.activateCount++
}
func (f *countingFacet) OnDeactivate() {}

func triggerTopBarButton(t *testing.T, bar *TopBarFacet, id string) {
	t.Helper()
	if bar == nil {
		t.Fatal("expected top bar")
	}
	bar.layout.OnArrange(gfx.RectFromXYWH(0, 0, 900, 32))
	rect, ok := bar.buttonRects[id]
	if !ok || rect.IsEmpty() {
		t.Fatalf("expected button rect for %s", id)
	}
	center := gfx.Point{X: (rect.Min.X + rect.Max.X) / 2, Y: (rect.Min.Y + rect.Max.Y) / 2}
	if !bar.input.OnPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: center, Button: platform.PointerLeft}) {
		t.Fatalf("expected press to be handled for %s", id)
	}
	if !bar.input.OnPointer(facet.PointerEvent{Kind: platform.PointerRelease, Position: center, Button: platform.PointerLeft}) {
		t.Fatalf("expected release to be handled for %s", id)
	}
}
