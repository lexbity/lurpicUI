package runtime

import (
	"image/color"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/theme"
)

func TestRuntime_materializesOnlyTargetedLayerPolicies(t *testing.T) {
	b := layout.NewLayerRegistryBuilder()
	layerARef := layout.LayerLayoutRecipeRef{Family: "app", Name: "layer-a"}
	layerBRef := layout.LayerLayoutRecipeRef{Family: "app", Name: "layer-b"}
	if _, err := b.RegisterLayer(layout.LayerRegistration{
		Name:          "a",
		Order:         100,
		WindowBinding: layout.WindowBinding{Kind: layout.WindowBindingPrimary},
		LayoutRecipe:  layerARef,
	}); err != nil {
		t.Fatalf("register layer a: %v", err)
	}
	if _, err := b.RegisterLayer(layout.LayerRegistration{
		Name:          "b",
		Order:         200,
		WindowBinding: layout.WindowBinding{Kind: layout.WindowBindingPrimary},
		LayoutRecipe:  layerBRef,
	}); err != nil {
		t.Fatalf("register layer b: %v", err)
	}
	reg, err := b.Freeze()
	if err != nil {
		t.Fatalf("freeze registry: %v", err)
	}

	resolver := theme.NewThemeResolver()
	countA := 0
	countB := 0
	if err := resolver.RegisterLayerLayoutRecipe(layerARef, func(ctx theme.ResolvedContext) layout.ResolvedLayerLayoutRecipe {
		countA++
		return layout.DefaultLayerLayoutRecipe()
	}); err != nil {
		t.Fatalf("register recipe a: %v", err)
	}
	if err := resolver.RegisterLayerLayoutRecipe(layerBRef, func(ctx theme.ResolvedContext) layout.ResolvedLayerLayoutRecipe {
		countB++
		return layout.DefaultLayerLayoutRecipe()
	}); err != nil {
		t.Fatalf("register recipe b: %v", err)
	}

	root := newRuntimeRenderFacet("root", gfx.RectFromXYWH(0, 0, 300, 200), color.RGBA{A: 255})
	child := newRuntimeRenderFacet("materialized", gfx.RectFromXYWH(0, 0, 40, 20), color.RGBA{R: 255, A: 255})
	rt, err := New(func() Config {
		cfg := DefaultConfig()
		cfg.LayerRegistry = reg
		cfg.ThemeResolver = resolver
		return cfg
	}(), nil, nil, &backendFixture{}, root)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	rt.window = &testWindow{width: 300, height: 200}
	rt.AddFacet(root, child, facet.Attachment{LayerID: facet.LayerID(reg.OrderedLayers()[0].ID)})
	rt.RunOneFrame()

	if countA != 1 {
		t.Fatalf("layer A recipe count = %d, want 1", countA)
	}
	if countB != 0 {
		t.Fatalf("layer B recipe count = %d, want 0 for unused layer", countB)
	}
	if _, ok := rt.ResolveProjectionLayer(child.Base().ID()); !ok {
		t.Fatal("expected materialized projection layer for targeted child")
	}
}
