package ui

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/marks/structure"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/ui_replay/model"
	"codeburg.org/lexbit/ui_replay/store"
)

func TestReplayResponsiveLayout(t *testing.T) {
	tablet := structure.ResponsiveLayoutForViewport(
		structure.Viewport{Width: 1280, Height: 1600},
		structure.Capabilities{Touch: true, IME: true},
	)
	if tablet.Variant != structure.ShellVariantTabletSplit {
		t.Fatalf("tablet.Variant = %v, want %v", tablet.Variant, structure.ShellVariantTabletSplit)
	}
	if tablet.Navigation != structure.NavigationTabs {
		t.Fatalf("tablet.Navigation = %v, want %v", tablet.Navigation, structure.NavigationTabs)
	}

	portrait := structure.ResponsiveLayoutForViewport(
		structure.Viewport{Width: 720, Height: 1280},
		structure.Capabilities{Touch: true, IME: true},
	)
	if portrait.Variant != structure.ShellVariantMobilePortrait {
		t.Fatalf("portrait.Variant = %v, want %v", portrait.Variant, structure.ShellVariantMobilePortrait)
	}

	model := TargetModel()
	visible := model.VisibleSurfaceIDs(portrait)
	if len(visible) != 2 || visible[0] != "content" || visible[1] != "header" {
		t.Fatalf("portrait visible surfaces = %v, want [content header]", visible)
	}
}

func TestReplayRootFacet_ResponsiveMobilePortraitCollapsesPanels(t *testing.T) {
	th := theme.Default()
	shaper := text.NewShaper(nil)
	shaper.SetContentScale(1.0)
	meta := model.DefaultBuildMetadata()

	root := NewReplayRootFacet(th, shaper, meta)
	bounds := gfx.RectFromXYWH(0, 0, 720, 1280)
	root.layout.OnArrange(bounds)

	if root.responsive.Variant != structure.ShellVariantMobilePortrait {
		t.Fatalf("responsive.Variant = %v, want %v", root.responsive.Variant, structure.ShellVariantMobilePortrait)
	}
	if root.shellBounds.Header.IsEmpty() {
		t.Fatal("expected header bounds to remain visible")
	}
	if root.shellBounds.Sidebar.IsEmpty() {
		t.Fatal("expected scenario sidebar bounds to remain visible")
	}
	if !root.shellBounds.Content.IsEmpty() {
		t.Fatal("expected content bounds to collapse in scenarios mode")
	}
	if !root.shellBounds.Inspector.IsEmpty() {
		t.Fatal("expected inspector bounds to collapse on mobile portrait")
	}
	if !root.shellBounds.Footer.IsEmpty() {
		t.Fatal("expected footer bounds to collapse on mobile portrait")
	}
}

func TestReplayRootFacet_MobileScenarioSelectionViaTouch(t *testing.T) {
	reg := store.NewScenarioRegistry()
	reg.Add(&model.Scenario{
		ID:          "alpha",
		DisplayName: "Alpha",
		Schema:      model.SchemaVersion,
		Actions:     []model.Action{{Type: model.ActionWaitFrames}},
	})
	reg.Add(&model.Scenario{
		ID:          "beta",
		DisplayName: "Beta",
		Schema:      model.SchemaVersion,
		Actions:     []model.Action{{Type: model.ActionWaitFrames}},
	})
	store.ScenarioRegistryStore.Set(reg)
	store.SelectedScenarioStore.Set("")
	store.ExecutionStateStore.Set(store.ExecutionState{})
	t.Cleanup(func() {
		store.ScenarioRegistryStore.Set(nil)
		store.SelectedScenarioStore.Set("")
		store.ExecutionStateStore.Set(store.ExecutionState{})
	})

	th := theme.Default()
	shaper := text.NewShaper(nil)
	shaper.SetContentScale(1.0)
	meta := model.DefaultBuildMetadata()

	root := NewReplayRootFacet(th, shaper, meta)
	facet.Attach(root, facet.AttachContext{})
	facet.Activate(root)
	root.layout.OnArrange(gfx.RectFromXYWH(0, 0, 720, 1280))

	root.sidebar.renderSidebar(&gfx.CommandList{}, root.shellBounds.Sidebar)
	betaRect, ok := root.sidebar.itemRects["beta"]
	if !ok || betaRect.IsEmpty() {
		t.Fatal("expected beta scenario item in mobile sidebar")
	}
	center := gfx.Point{X: (betaRect.Min.X + betaRect.Max.X) / 2, Y: (betaRect.Min.Y + betaRect.Max.Y) / 2}
	if !root.sidebar.input.OnPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: center, Button: platform.PointerLeft}) {
		t.Fatal("expected beta press to be handled")
	}
	if !root.sidebar.input.OnPointer(facet.PointerEvent{Kind: platform.PointerRelease, Position: center, Button: platform.PointerLeft}) {
		t.Fatal("expected beta release to be handled")
	}
	if got := store.SelectedScenarioStore.Get(); got != "beta" {
		t.Fatalf("expected beta selected, got %q", got)
	}
	if got, ok := root.GetSelectedScenario(); !ok || got == nil || got.ID != "beta" {
		t.Fatalf("expected beta selected scenario, got %#v", got)
	}
}

func TestReplayRootFacet_ExecutionStatePersistsAcrossLayoutChanges(t *testing.T) {
	exec := store.ExecutionState{
		Status:        model.StatusRunning,
		CurrentStep:   2,
		TotalSteps:    5,
		CurrentAction: "wait",
		Progress:      0.4,
	}
	store.ExecutionStateStore.Set(exec)
	t.Cleanup(func() {
		store.ExecutionStateStore.Set(store.ExecutionState{})
	})

	th := theme.Default()
	shaper := text.NewShaper(nil)
	shaper.SetContentScale(1.0)
	meta := model.DefaultBuildMetadata()

	root := NewReplayRootFacet(th, shaper, meta)
	facet.Attach(root, facet.AttachContext{})
	facet.Activate(root)

	root.layout.OnArrange(gfx.RectFromXYWH(0, 0, 1440, 900))
	root.layout.OnArrange(gfx.RectFromXYWH(0, 0, 720, 1280))
	root.layout.OnArrange(gfx.RectFromXYWH(0, 0, 1440, 900))

	got := store.ExecutionStateStore.Get()
	if got.Status != exec.Status || got.CurrentStep != exec.CurrentStep || got.TotalSteps != exec.TotalSteps || got.CurrentAction != exec.CurrentAction {
		t.Fatalf("execution state changed across layout updates: got %#v want %#v", got, exec)
	}
}

func TestReplayRootFacet_MobileArtifactsViewShowsInspectorAndFooter(t *testing.T) {
	th := theme.Default()
	shaper := text.NewShaper(nil)
	shaper.SetContentScale(1.0)
	meta := model.DefaultBuildMetadata()

	root := NewReplayRootFacet(th, shaper, meta)
	facet.Attach(root, facet.AttachContext{})
	facet.Activate(root)
	root.setMobilePanel("artifacts")
	root.layout.OnArrange(gfx.RectFromXYWH(0, 0, 720, 1280))

	if root.shellBounds.Inspector.IsEmpty() {
		t.Fatal("expected inspector bounds in artifacts mode")
	}
	if root.shellBounds.Footer.IsEmpty() {
		t.Fatal("expected footer bounds in artifacts mode")
	}
	if !root.shellBounds.Sidebar.IsEmpty() {
		t.Fatal("expected sidebar to collapse in artifacts mode")
	}
	if !root.shellBounds.Content.IsEmpty() {
		t.Fatal("expected content to collapse in artifacts mode")
	}
}
