package uinav

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/theme"
)

func TestResolveMenuRecipe_reports_dense_variant(t *testing.T) {
	slots, report := ResolveMenuRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()}, MenuDense)
	if report.Variant != theme.VariantKey("dense") {
		t.Fatalf("variant = %q, want dense", report.Variant)
	}
	if source, ok := report.SlotSource("Surface"); !ok || source != theme.SlotSourceVariantDefault {
		t.Fatalf("surface source = %v, %v", source, ok)
	}
	if slots.Surface.Base.Fills == nil {
		t.Fatal("expected dense menu surface styling")
	}
}

func TestResolveDrawerAndSpeedDialRecipes_expose_expected_slots(t *testing.T) {
	drawer, drawerReport := ResolveDrawerRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()})
	if drawerReport.Variant != theme.VariantKey("standard") {
		t.Fatalf("drawer variant = %q, want standard", drawerReport.Variant)
	}
	if _, ok := drawerReport.SlotSource("Surface"); !ok {
		t.Fatal("expected drawer surface slot source")
	}
	if drawer.Surface.Base.Fills == nil || drawer.Scrim.Base.Fills == nil {
		t.Fatal("expected drawer slots to be populated")
	}

	speedDial, speedDialReport := ResolveSpeedDialRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()})
	if speedDialReport.Variant != theme.VariantKey("standard") {
		t.Fatalf("speed dial variant = %q, want standard", speedDialReport.Variant)
	}
	if _, ok := speedDialReport.SlotSource("Fab"); !ok {
		t.Fatal("expected speed dial fab slot source")
	}
	if speedDial.Fab.Base.Fills == nil || speedDial.Backdrop.Base.Fills == nil {
		t.Fatal("expected speed dial slots to be populated")
	}
}
