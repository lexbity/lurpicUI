package templates

import "testing"

func TestDefaultRecipeBundle_catalog(t *testing.T) {
	bundle := DefaultRecipeBundle()
	if got := bundle.BundleNames(); len(got) != 5 {
		t.Fatalf("bundle names = %v", got)
	}
	if family, ok := bundle.Lookup("uiinput"); !ok || family.Family != "uiinput" {
		t.Fatalf("lookup uiinput failed: %#v %v", family, ok)
	}
	if got := DeclaredVariants(AnnotationRecipeBundle()); len(got) != 3 {
		t.Fatalf("annotation variants = %v", got)
	}
}

func TestDefaultTemplateTheme_validateAndDiagnostics(t *testing.T) {
	theme := DefaultTemplateTheme("notes")
	if err := theme.Validate(); err != nil {
		t.Fatalf("validate failed: %v", err)
	}

	diag := theme.Diagnostics(DensityRegular)
	if diag.ThemeName != "notes" {
		t.Fatalf("theme name = %q", diag.ThemeName)
	}
	if !diag.HasMaterialRegistry {
		t.Fatal("expected default template to include a material registry")
	}
	if diag.ChartOverride {
		t.Fatal("expected no chart override in default template")
	}
	if len(diag.BundleNames) != 5 {
		t.Fatalf("bundle names = %v", diag.BundleNames)
	}
}

func TestRecipeContext_applies_density_scaling(t *testing.T) {
	theme := DefaultTemplateTheme("notes")
	ctx := theme.ResolveInputs(DensityCompact)
	if ctx.Typography.BodyMedium.Size != 13.02 {
		t.Fatalf("body size = %v", ctx.Typography.BodyMedium.Size)
	}
	if ctx.Metrics.Control.Height != 32 {
		t.Fatalf("control height = %v", ctx.Metrics.Control.Height)
	}
	if ctx.Fonts.UISans.DefaultStyle.Size == 0 {
		t.Fatal("expected resolved font defaults")
	}
}
