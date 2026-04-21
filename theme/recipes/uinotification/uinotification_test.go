package uinotification

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/theme"
)

func TestResolveSnackbarRecipe_reports_slots(t *testing.T) {
	slots, report := ResolveSnackbarRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()})
	if report.Variant != theme.VariantKey("standard") {
		t.Fatalf("variant = %q, want standard", report.Variant)
	}
	if _, ok := report.SlotSource("Container"); !ok {
		t.Fatal("expected snackbar container slot source")
	}
	if slots.Container.Base.Fills == nil || slots.Action.Base.Fills == nil {
		t.Fatal("expected snackbar slots to be populated")
	}
}

func TestResolveDialogRecipe_variants_reported(t *testing.T) {
	slots, report := ResolveDialogRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()}, DialogDestructive)
	if report.Variant != theme.VariantKey("destructive") {
		t.Fatalf("variant = %q, want destructive", report.Variant)
	}
	if slots.Scrim.Base.Fills == nil || slots.Outline.Base.Strokes == nil {
		t.Fatal("expected dialog slots to be populated")
	}
}

func TestResolveProgressRecipe_reports_slots(t *testing.T) {
	slots, report := ResolveProgressRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()})
	if _, ok := report.SlotSource("Track"); !ok {
		t.Fatal("expected progress track slot source")
	}
	if slots.Track.Base.Fills == nil || slots.Indicator.Base.Fills == nil {
		t.Fatal("expected progress slots to be populated")
	}
}
