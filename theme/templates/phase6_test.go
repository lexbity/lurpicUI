package templates

import (
	"strings"
	"testing"
)

func TestValidationReport_detectsMissingTokens(t *testing.T) {
	theme := TemplateTheme{
		Name: "broken",
		Tokens: Tokens{
			Color: ColorTokens{},
		},
		Fonts:   DefaultFontRoles(),
		Recipes: DefaultRecipeBundle(),
		Metadata: ThemeMetadata{
			BaselineDensity: DensityRegular,
			SupportsRegular: true,
		},
	}

	report := theme.ValidationReport(DensityRegular)
	if len(report.MissingTokens) == 0 {
		t.Fatal("expected missing tokens to be reported")
	}
	if !containsString(report.MissingTokens, "Color.DataPalette") {
		t.Fatalf("missing tokens = %v", report.MissingTokens)
	}
	if !containsString(report.MissingTokens, "Typography.BodyMedium") {
		t.Fatalf("missing tokens = %v", report.MissingTokens)
	}
	if err := report.Error(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidationReport_detectsMissingRecipesAndDensityIssues(t *testing.T) {
	theme := DefaultTemplateTheme("broken")
	theme.Recipes = RecipeBundle{}
	theme.Tokens.Metrics.Control.Height = DensityTriplet{Compact: 48, Regular: 32, Touchspread: 24}

	report := theme.ValidationReport(DensityRegular)
	if !containsString(report.MissingRecipes, "annotation") {
		t.Fatalf("missing recipes = %v", report.MissingRecipes)
	}
	if len(report.InvalidDensityScaling) == 0 {
		t.Fatal("expected invalid density scaling to be reported")
	}
	if !containsSubstring(report.InvalidDensityScaling, "Metric.Control.Height") {
		t.Fatalf("density issues = %v", report.InvalidDensityScaling)
	}
	if err := theme.Validate(); err == nil {
		t.Fatal("expected validation failure")
	}
}

func TestDiagnostics_reportsChartFallbackMarkers(t *testing.T) {
	diag := DefaultTemplateTheme("notes").Diagnostics(DensityRegular)
	if !diag.ChartUsesFallback {
		t.Fatal("expected chart fallback to be reported")
	}
	if len(diag.ChartFallbackFields) == 0 {
		t.Fatal("expected fallback fields")
	}

	shipped := Notes().Diagnostics(DensityRegular)
	if shipped.ChartUsesFallback {
		t.Fatal("notes should not require chart fallbacks")
	}
	if len(shipped.ChartFallbackFields) != 0 {
		t.Fatalf("unexpected fallback fields = %v", shipped.ChartFallbackFields)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func containsSubstring(values []string, want string) bool {
	for _, value := range values {
		if strings.Contains(value, want) {
			return true
		}
	}
	return false
}
