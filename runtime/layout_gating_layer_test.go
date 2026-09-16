package runtime

import (
	"image/color"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/theme"
)

// TestRuntimeResolveAttachedLayersCascadesEmptyWhenHostGated pins
// F-inactive-layer-child: when a host is gated to empty bounds, its
// layer-attached children must stay inert (arranged to empty + an empty
// projection layer), not be resurrected by the layer pass's window/MeasuredSize
// parentBounds fallback. Pre-fix, the free-layer trigger below would land at
// (seed.X, seed.Y) = (240,160) and steal hits from the active exhibit.
func TestRuntimeResolveAttachedLayersCascadesEmptyWhenHostGated(t *testing.T) {
	// A free-recipe layer, mirroring how the studio registers studio.trigger.
	freeRef := layout.LayerLayoutRecipeRef{Family: "gating-test", Name: "free-trigger"}
	b := layout.NewLayerRegistryBuilder()
	layerID, err := b.RegisterLayer(layout.LayerRegistration{
		Name:         "trigger",
		Order:        100,
		HitPolicy:    layout.HitNormal,
		ClipPolicy:   layout.ClipNone,
		LayoutRecipe: freeRef,
	})
	if err != nil {
		t.Fatalf("register layer: %v", err)
	}
	reg, err := b.Freeze()
	if err != nil {
		t.Fatalf("freeze registry: %v", err)
	}
	resolver := theme.NewThemeResolver()
	if err := resolver.RegisterLayerLayoutRecipe(freeRef, func(theme.ResolvedContext) layout.ResolvedLayerLayoutRecipe {
		return layout.ResolvedLayerLayoutRecipe{PolicyKind: layout.LayerLayoutFree}
	}); err != nil {
		t.Fatalf("register recipe: %v", err)
	}

	// A host that gates ITSELF to empty bounds on arrange (its OnArrange records
	// empty ArrangedBounds), mirroring an exhibit the Stage has hidden.
	host := newRuntimeRenderFacet("host", gfx.RectFromXYWH(0, 0, 100, 100), color.RGBA{A: 255})
	host.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		host.layout.ArrangedBounds = gfx.Rect{} // host is gated/hidden
	}
	// A free-layer child of the host: positioned at its free offset, which with
	// an empty parentBounds would resurrect it at (240,160).
	trigger := newRuntimeRenderFacet("trigger", gfx.RectFromXYWH(0, 0, 56, 56), color.RGBA{R: 255, A: 255})
	trigger.layout.Child.SupportedPlacement = facet.SupportsFree

	rt, err := New(func() Config {
		cfg := DefaultConfig()
		cfg.LayerRegistry = reg
		cfg.ThemeResolver = resolver
		return cfg
	}(), nil, nil, &backendFixture{}, host)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	rt.window = &testWindow{width: 400, height: 300}
	rt.AddFacet(host, trigger, facet.Attachment{
		LayerID: facet.LayerID(layerID),
		Placement: facet.Placement{
			Mode: facet.PlacementFree,
			Free: facet.FreePlacement{
				X: facet.ResolvedScalar(240),
				Y: facet.ResolvedScalar(160),
			},
		},
	})
	rt.RunOneFrame()

	// The trigger must be inert: empty arranged bounds ...
	if got := trigger.layout.ArrangedBounds; !got.IsEmpty() {
		t.Fatalf("gated-host trigger bounds = %v, want empty (F-inactive-layer-child)", got)
	}
	// ... and an empty projection layer (no window-coords resurrection).
	layer, ok := rt.ResolveProjectionLayer(trigger.Base().ID())
	if !ok {
		t.Fatal("expected a projection layer entry for the gated trigger")
	}
	if !layer.Bounds.IsEmpty() {
		t.Fatalf("gated-host trigger projection layer bounds = %v, want empty", layer.Bounds)
	}
}
