package theme

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/fontdata"
	"codeburg.org/lexbit/lurpicui/layout"
)

func TestDefaultResolvedContext_providesContextAndDensity(t *testing.T) {
	ctx := DefaultResolvedContext()

	if got := ctx.Color(ColorBackground); got == (gfx.Color{}) {
		t.Fatal("background color should not be zero")
	}
	if got := ctx.Spacing(SpacingXS).Float32(); got <= 0 {
		t.Fatalf("expected positive spacing, got %v", got)
	}
	if got := ctx.TextStyle(TextBodyM); got.Size <= 0 {
		t.Fatalf("expected positive body text size, got %v", got.Size)
	}
	if ctx.Density.ID == "" {
		t.Fatal("expected default density id")
	}
	if got := ctx.TextStyle(TextBodyM); got.Family == "" {
		t.Fatal("expected resolved body family")
	}
}

func TestDensityScale_resolvesSpacingAndTypography(t *testing.T) {
	tokens := DefaultTokens()
	scale := DefaultDensityScale(DensityIDCompact, tokens)
	if scale.ID != DensityIDCompact {
		t.Fatalf("density id = %q", scale.ID)
	}
	if got := scale.ResolveSpacing(SpacingM); got <= 0 {
		t.Fatalf("spacing should be positive, got %v", got)
	}
	if got := scale.ResolveTextStyle(TextBodyM); got.Size <= 0 {
		t.Fatalf("text style should be positive, got %#v", got)
	}
	if got := scale.Scale(10); got <= 0 {
		t.Fatalf("scaled value should be positive, got %v", got)
	}
}

func TestStateRecipeCatalog_fallbackOrder(t *testing.T) {
	catalog := NewStateRecipeCatalog[string]()
	catalog.RegisterExact("uiinput", "button/filled", RecipeStateHover, "exact-hover")
	catalog.RegisterExact("uiinput", "button/filled", RecipeStateDefault, "exact-default")
	catalog.RegisterFamilyDefault("uiinput", RecipeStateHover, "family-hover")
	catalog.RegisterGlobalDefault(RecipeStateDefault, "global-default")

	if got, ok := catalog.Resolve("uiinput", "button/filled", RecipeStateHover); !ok || got != "exact-hover" {
		t.Fatalf("exact hover fallback failed: %q %v", got, ok)
	}
	if got, ok := catalog.Resolve("uiinput", "button/filled", RecipeStateActive); !ok || got != "exact-default" {
		t.Fatalf("exact default fallback failed: %q %v", got, ok)
	}
	if got, ok := catalog.Resolve("uiinput", "missing", RecipeStateHover); !ok || got != "family-hover" {
		t.Fatalf("family fallback failed: %q %v", got, ok)
	}
	if got, ok := catalog.Resolve("missing", "missing", RecipeStateInvalid); !ok || got != "global-default" {
		t.Fatalf("global fallback failed: %q %v", got, ok)
	}
}

func TestThemeResolver_layerAndGroupRecipes(t *testing.T) {
	resolver := NewThemeResolver()
	layerRef := layout.LayerLayoutRecipeRef{Family: "app", Name: "canvas"}
	groupRef := layout.GroupLayoutRecipeRef{Family: "app", Name: "panel"}

	if err := resolver.RegisterLayerLayoutRecipe(layerRef, func(ctx ResolvedContext) layout.ResolvedLayerLayoutRecipe {
		return layout.ResolvedLayerLayoutRecipe{PolicyKind: layout.LayerLayoutGrid, Grid: layout.ResolvedGridConfig{Columns: 3, Rows: 2}, Insets: gfx.Insets{Top: 1}}
	}); err != nil {
		t.Fatalf("register layer recipe: %v", err)
	}
	if err := resolver.RegisterGroupLayoutRecipe(groupRef, func(ctx ResolvedContext) layout.ResolvedGroupLayoutRecipe {
		return layout.ResolvedGroupLayoutRecipe{PolicyKind: layout.GroupLayoutGrid, Grid: layout.ResolvedGridConfig{Columns: 2, Rows: 2}, Overflow: layout.OverflowClip}
	}); err != nil {
		t.Fatalf("register group recipe: %v", err)
	}

	ctx := DefaultResolvedContext().WithResolver(resolver)
	if got, ok := ctx.ResolveLayerLayoutRecipe(layerRef); !ok || got.PolicyKind != layout.LayerLayoutGrid || got.Grid.Columns != 3 || got.Grid.Rows != 2 {
		t.Fatalf("layer recipe resolution failed: %#v %v", got, ok)
	}
	if got, ok := ctx.ResolveGroupLayoutRecipe(groupRef); !ok || got.PolicyKind != layout.GroupLayoutGrid || got.Grid.Columns != 2 || got.Overflow != layout.OverflowClip {
		t.Fatalf("group recipe resolution failed: %#v %v", got, ok)
	}
	if _, ok := ctx.ResolveLayerLayoutRecipe(layout.LayerLayoutRecipeRef{Family: "missing", Name: "missing"}); ok {
		t.Fatal("expected missing layer recipe lookup to fail")
	}
}

func TestResolvedContext_uses_font_registry_for_text_styles(t *testing.T) {
	ctx := DefaultResolvedContext().WithFontRegistry(fontdata.TestFontRegistry(t))
	style := ctx.TextStyle(TextBodyM)
	if style.Family != "Noto Sans" {
		t.Fatalf("expected loaded family to win, got %q", style.Family)
	}
}
