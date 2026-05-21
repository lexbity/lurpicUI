package templates

import (
	"math"
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
)

func TestDefaultTypographyTokens_matchesPlan(t *testing.T) {
	typography := DefaultTypographyTokens()
	if typography.DisplayLarge.Size != 36 || typography.DisplayLarge.LineHeight != 44 {
		t.Fatalf("unexpected display large: %#v", typography.DisplayLarge)
	}
	if typography.LabelMedium.Weight != text.WeightSemiBold {
		t.Fatalf("unexpected label medium weight: %#v", typography.LabelMedium)
	}
	if typography.MonoSmall.Size != 12 || typography.MonoSmall.LineHeight != 18 {
		t.Fatalf("unexpected mono small: %#v", typography.MonoSmall)
	}
}

func TestScaleTypographyForDensity(t *testing.T) {
	base := DefaultTypographyTokens()
	scaled := ScaleTypographyForDensity(base, DensityCompact)
	if got := scaled.BodyMedium.Size; math.Abs(float64(got-13.02)) > 0.001 {
		t.Fatalf("compact size = %v", got)
	}
	if got := scaled.BodyMedium.LineHeight; math.Abs(float64(got-20.46)) > 0.001 {
		t.Fatalf("compact line height = %v", got)
	}
}

func TestScaleMetricsForDensity(t *testing.T) {
	metrics := DefaultMetricTokens()
	resolved := ScaleMetricsForDensity(metrics, DensityTouchspread)
	if got := resolved.Control.Height; got != 48 {
		t.Fatalf("control height = %v", got)
	}
	if got := resolved.Navigation.MenuRowHeight; got != 44 {
		t.Fatalf("menu row height = %v", got)
	}
	if got := resolved.Notification.DialogMaxWidth; got != 760 {
		t.Fatalf("dialog max width = %v", got)
	}
}

func TestFontRoleResolveUsesFallbackChain(t *testing.T) {
	role := theme.FontRole{
		PreferredFamilies: []string{"Missing Font", "Also Missing"},
		DefaultStyle:      text.TextStyle{Size: 14, Weight: text.WeightRegular},
	}
	got := role.Resolve(nil)
	if got.Family != "Missing Font" {
		t.Fatalf("expected first preferred family, got %q", got.Family)
	}
}

func TestTemplateThemeValidate_requiresFonts(t *testing.T) {
	theme := TemplateTheme{
		Name: "notes",
		Tokens: Tokens{
			Color: ColorTokens{
				DataPalette: []gfx.Color{gfx.ColorFromRGBA8(1, 2, 3, 255)},
			},
		},
		Metadata: ThemeMetadata{
			BaselineDensity: DensityRegular,
			SupportsRegular: true,
		},
	}
	if err := theme.Validate(); err == nil {
		t.Fatal("expected validation error for missing font roles")
	}
}
