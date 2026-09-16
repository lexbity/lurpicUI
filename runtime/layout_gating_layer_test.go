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
// F-inactive-layer-child: when a host is gated to empty bounds by its parent,
// its layer-attached children must stay inert (arranged to empty + an empty
// projection layer), not be resurrected by the layer pass's window/MeasuredSize
// parentBounds fallback. Pre-fix, the free-layer trigger below would land at
// (seed.X, seed.Y) = (240,160) and steal hits from the active exhibit.
//
// The runtime root itself is excluded from gating (empty root bounds mean
// "not yet arranged", not "hidden" — see resolveAttachedLayers), so the
// fixture gates a host under a plain container root: the same shape as a
// Stage arranging an inactive exhibit to gfx.Rect{} (see studio e1_grid.go).
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

	// A plain container root. It is never gated: as the runtime root it has no
	// gating parent, so its empty bounds mean "not yet arranged".
	root := newRuntimeRenderFacet("root", gfx.RectFromXYWH(0, 0, 400, 300), color.RGBA{G: 40, A: 255})
	// A host the ROOT gates to empty bounds on arrange, mirroring an exhibit
	// the Stage has hidden (parent-driven gating).
	host := newRuntimeRenderFacet("host", gfx.RectFromXYWH(0, 0, 100, 100), color.RGBA{A: 255})
	root.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		root.layout.ArrangedBounds = bounds
		host.layout.Arrange(facet.ArrangeContext{}, gfx.Rect{}) // root hides the host
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
	}(), nil, nil, &backendFixture{}, root)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	rt.window = &testWindow{width: 400, height: 300}
	rt.AddFacet(root, host, facet.Attachment{})
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

// TestRuntimeResolveAttachedLayersRootEmptyBoundsDoNotGate pins the other half
// of the gating contract: the runtime root is excluded from
// F-inactive-layer-child. Empty bounds on the root mean "not yet arranged"
// (a first frame), not "hidden" — its layer-attached children must still be
// resolved through the parentBounds fallback so a first frame lays out
// overlays instead of cascading everything to empty.
func TestRuntimeResolveAttachedLayersRootEmptyBoundsDoNotGate(t *testing.T) {
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

	// A runtime root whose arranged bounds are still empty (as on a first
	// frame, before its arrange has landed).
	root := newRuntimeRenderFacet("root", gfx.RectFromXYWH(0, 0, 100, 100), color.RGBA{A: 255})
	root.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		root.layout.ArrangedBounds = gfx.Rect{}
	}
	trigger := newRuntimeRenderFacet("trigger", gfx.RectFromXYWH(0, 0, 56, 56), color.RGBA{R: 255, A: 255})
	trigger.layout.Child.SupportedPlacement = facet.SupportsFree

	rt, err := New(func() Config {
		cfg := DefaultConfig()
		cfg.LayerRegistry = reg
		cfg.ThemeResolver = resolver
		return cfg
	}(), nil, nil, &backendFixture{}, root)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	rt.window = &testWindow{width: 400, height: 300}
	rt.AddFacet(root, trigger, facet.Attachment{
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

	// The fallback (MeasuredSize → window size) must position the trigger at
	// its free seed offset; an empty result would mean the root was gated,
	// which would blank every overlay on first frame.
	if got := trigger.layout.ArrangedBounds; got.IsEmpty() {
		t.Fatalf("unarranged-root trigger bounds = %v, want the free-seed fallback (root is excluded from gating)", got)
	}
	if got := trigger.layout.ArrangedBounds.Min; got != (gfx.Point{X: 240, Y: 160}) {
		t.Fatalf("unarranged-root trigger bounds Min = %v, want (240,160)", got)
	}
	layer, ok := rt.ResolveProjectionLayer(trigger.Base().ID())
	if !ok {
		t.Fatal("expected a projection layer entry for the trigger")
	}
	if layer.Bounds.IsEmpty() {
		t.Fatalf("unarranged-root trigger projection layer bounds = %v, want non-empty", layer.Bounds)
	}
}
