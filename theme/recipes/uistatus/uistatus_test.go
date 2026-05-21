package uistatus

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/theme"
)

func TestResolveBadgeRecipe_exposes_expected_slots(t *testing.T) {
	slots, report := ResolveBadgeRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()}, BadgeDefault)
	if report.Variant != theme.VariantKey("default") {
		t.Fatalf("variant = %q, want default", report.Variant)
	}
	for _, name := range []string{"Root", "BadgeContainer", "Label", "OptionalIcon"} {
		if _, ok := report.SlotSource(name); !ok {
			t.Fatalf("expected badge slot source for %s", name)
		}
	}
	if slots.Root.Base.Opacity != 0 {
		t.Fatal("expected transparent root slot")
	}
	if slots.BadgeContainer.Base.Fills == nil {
		t.Fatal("expected badge container fill")
	}
	if slots.Label.Base.Fills == nil {
		t.Fatal("expected label mark style")
	}
}

func TestResolveBadgeRecipe_disabled_variant(t *testing.T) {
	slots, report := ResolveBadgeRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()}, BadgeDisabled)
	if report.Variant != theme.VariantKey("disabled") {
		t.Fatalf("variant = %q, want disabled", report.Variant)
	}
	if slots.BadgeContainer.Base.Fills == nil {
		t.Fatal("expected badge container fill")
	}
	if slots.Label.Base.Fills == nil {
		t.Fatal("expected disabled label mark style")
	}
}

func TestResolveStatusLightRecipe_exposes_expected_slots(t *testing.T) {
	slots, report := ResolveStatusLightRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()}, StatusLightDefault)
	if report.Variant != theme.VariantKey("default") {
		t.Fatalf("variant = %q, want default", report.Variant)
	}
	for _, name := range []string{"Root", "Indicator", "LabelOptional"} {
		if _, ok := report.SlotSource(name); !ok {
			t.Fatalf("expected status-light slot source for %s", name)
		}
	}
	if slots.Root.Base.Opacity != 0 {
		t.Fatal("expected transparent root slot")
	}
	if slots.Indicator.Base.Fills == nil {
		t.Fatal("expected indicator fill")
	}
}

func TestResolveStatusLightRecipe_disabled_variant(t *testing.T) {
	slots, report := ResolveStatusLightRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()}, StatusLightDisabled)
	if report.Variant != theme.VariantKey("disabled") {
		t.Fatalf("variant = %q, want disabled", report.Variant)
	}
	if slots.Indicator.Base.Fills == nil {
		t.Fatal("expected disabled indicator fill")
	}
}

func TestResolveProgressBarRecipe_exposes_expected_slots(t *testing.T) {
	slots, report := ResolveProgressBarRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()}, ProgressBarDefault)
	if report.Variant != theme.VariantKey("default") {
		t.Fatalf("variant = %q, want default", report.Variant)
	}
	for _, name := range []string{"Root", "Track", "Indicator", "OptionalLabel"} {
		if _, ok := report.SlotSource(name); !ok {
			t.Fatalf("expected progress-bar slot source for %s", name)
		}
	}
	if slots.Root.Base.Opacity <= 0 {
		t.Fatal("expected visible root slot")
	}
	if slots.Track.Base.Fills == nil {
		t.Fatal("expected track fill")
	}
	if slots.Indicator.Base.Fills == nil {
		t.Fatal("expected indicator fill")
	}
}

func TestResolveProgressBarRecipe_disabled_variant(t *testing.T) {
	slots, report := ResolveProgressBarRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()}, ProgressBarDisabled)
	if report.Variant != theme.VariantKey("disabled") {
		t.Fatalf("variant = %q, want disabled", report.Variant)
	}
	if slots.Indicator.Base.Fills == nil {
		t.Fatal("expected disabled indicator fill")
	}
}

func TestResolveProgressRingRecipe_exposes_expected_slots(t *testing.T) {
	slots, report := ResolveProgressRingRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()}, ProgressRingDefault)
	if report.Variant != theme.VariantKey("default") {
		t.Fatalf("variant = %q, want default", report.Variant)
	}
	for _, name := range []string{"Root", "TrackArc", "IndicatorArc", "OptionalLabel"} {
		if _, ok := report.SlotSource(name); !ok {
			t.Fatalf("expected progress-ring slot source for %s", name)
		}
	}
	if slots.Root.Base.Opacity <= 0 {
		t.Fatal("expected visible root slot")
	}
	if slots.TrackArc.Base.Fills == nil {
		t.Fatal("expected track arc fill")
	}
	if slots.IndicatorArc.Base.Fills == nil {
		t.Fatal("expected indicator arc fill")
	}
}

func TestResolveProgressRingRecipe_disabled_variant(t *testing.T) {
	slots, report := ResolveProgressRingRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()}, ProgressRingDisabled)
	if report.Variant != theme.VariantKey("disabled") {
		t.Fatalf("variant = %q, want disabled", report.Variant)
	}
	if slots.IndicatorArc.Base.Fills == nil {
		t.Fatal("expected disabled indicator arc fill")
	}
}
