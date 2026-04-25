package structure

import "testing"

func TestResponsiveLayoutForViewport(t *testing.T) {
	t.Parallel()

	desktop := ResponsiveLayoutForViewport(Viewport{Width: 1600, Height: 900}, Capabilities{Hover: true, Keyboard: true})
	if desktop.Variant != ShellVariantDesktopDense {
		t.Fatalf("desktop.Variant = %v, want %v", desktop.Variant, ShellVariantDesktopDense)
	}
	if !desktop.ShowHoverHints {
		t.Fatal("desktop.ShowHoverHints = false, want true")
	}
	if desktop.MinHitTarget != 32 {
		t.Fatalf("desktop.MinHitTarget = %v, want 32", desktop.MinHitTarget)
	}

	mobile := ResponsiveLayoutForViewport(Viewport{Width: 720, Height: 1280}, Capabilities{Touch: true, IME: true})
	if mobile.Variant != ShellVariantMobilePortrait {
		t.Fatalf("mobile.Variant = %v, want %v", mobile.Variant, ShellVariantMobilePortrait)
	}
	if mobile.Navigation != NavigationDrawer {
		t.Fatalf("mobile.Navigation = %v, want %v", mobile.Navigation, NavigationDrawer)
	}
	if mobile.ShowHoverHints {
		t.Fatal("mobile.ShowHoverHints = true, want false")
	}
	if mobile.MinHitTarget != 56 {
		t.Fatalf("mobile.MinHitTarget = %v, want 56", mobile.MinHitTarget)
	}
}

func TestTargetModelSurfaceNavigation(t *testing.T) {
	t.Parallel()

	model := TargetModel{
		AppID: "demo",
		Surfaces: []SurfaceSpec{
			{ID: "primary-a", Role: SurfacePrimary},
			{ID: "secondary-a", Role: SurfaceSecondary},
			{ID: "optional-a", Role: SurfaceOptional},
		},
	}

	layout := ResponsiveLayout{Variant: ShellVariantMobilePortrait}
	visible := model.VisibleSurfaceIDs(layout)
	if len(visible) != 1 || visible[0] != "primary-a" {
		t.Fatalf("VisibleSurfaceIDs() = %v, want [primary-a]", visible)
	}

	navigator := NewSurfaceNavigator(model, layout)
	if !navigator.Contains("primary-a") {
		t.Fatal("navigator should contain primary-a")
	}
	if got := navigator.Next("primary-a"); got != "primary-a" {
		t.Fatalf("navigator.Next(primary-a) = %q, want primary-a", got)
	}
}

func TestPanelToggleState(t *testing.T) {
	t.Parallel()

	var state PanelToggleState
	if state.Visible(false) {
		t.Fatal("collapsed state should not be visible when defaultVisible is false")
	}

	state.Toggle()
	if !state.Visible(false) {
		t.Fatal("expanded state should be visible")
	}

	state.Collapse()
	if state.Visible(false) {
		t.Fatal("collapsed state should not be visible")
	}
}
