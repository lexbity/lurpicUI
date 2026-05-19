package runtime

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/projection"
)

func TestRuntimeNew_primaryWindowBinding_defaulted(t *testing.T) {
	b := layout.NewLayerRegistryBuilder()
	if err := b.RegisterStandardLayers(); err != nil {
		t.Fatalf("register standard layers: %v", err)
	}
	primaryID, err := b.RegisterLayer(layout.LayerRegistration{
		Name:          "app.primary",
		Order:         2500,
		WindowBinding: layout.WindowBinding{Kind: layout.WindowBindingPrimary},
	})
	if err != nil {
		t.Fatalf("register layer: %v", err)
	}
	reg, err := b.Freeze()
	if err != nil {
		t.Fatalf("freeze: %v", err)
	}
	cfg := DefaultConfig()
	cfg.LayerRegistry = reg
	root := facet.NewFacet()
	rt, err := New(cfg, &nilApp{}, &testWindow{width: 640, height: 480}, &backendFixture{}, &root)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	if got := rt.windowBindings[windowBindingKey(layout.WindowBinding{Kind: layout.WindowBindingPrimary})]; got == nil {
		t.Fatal("primary window binding not installed")
	}
	if _, ok := rt.layerRegistry.Lookup(primaryID); !ok {
		t.Fatal("primary layer missing from registry")
	}
}

func TestRuntimeNew_rejects_unknown_named_windowBinding(t *testing.T) {
	b := layout.NewLayerRegistryBuilder()
	if err := b.RegisterStandardLayers(); err != nil {
		t.Fatalf("register standard layers: %v", err)
	}
	if _, err := b.RegisterLayer(layout.LayerRegistration{
		Name:          "app.tools",
		Order:         2600,
		WindowBinding: layout.WindowBinding{Kind: layout.WindowBindingNamed, Name: "tools"},
	}); err != nil {
		t.Fatalf("register layer: %v", err)
	}
	reg, err := b.Freeze()
	if err != nil {
		t.Fatalf("freeze: %v", err)
	}
	cfg := DefaultConfig()
	cfg.LayerRegistry = reg
	root := facet.NewFacet()
	if _, err := New(cfg, &nilApp{}, &testWindow{width: 640, height: 480}, &backendFixture{}, &root); err == nil {
		t.Fatal("expected named window binding rejection")
	}
}

func TestRuntime_assembleWindowFrames_groups_by_binding(t *testing.T) {
	b := layout.NewLayerRegistryBuilder()
	if err := b.RegisterStandardLayers(); err != nil {
		t.Fatalf("register standard layers: %v", err)
	}
	_, err := b.RegisterLayer(layout.LayerRegistration{
		Name:          "app.primary",
		Order:         2500,
		WindowBinding: layout.WindowBinding{Kind: layout.WindowBindingPrimary},
	})
	if err != nil {
		t.Fatalf("register primary: %v", err)
	}
	_, err = b.RegisterLayer(layout.LayerRegistration{
		Name:          "app.tools",
		Order:         2600,
		WindowBinding: layout.WindowBinding{Kind: layout.WindowBindingNamed, Name: "tools"},
	})
	if err != nil {
		t.Fatalf("register tools: %v", err)
	}
	reg, err := b.Freeze()
	if err != nil {
		t.Fatalf("freeze: %v", err)
	}
	rt := &Runtime{
		layerRegistry: reg,
		windowBindings: map[string]platform.Window{
			windowBindingKey(layout.WindowBinding{Kind: layout.WindowBindingPrimary}): &testWindow{width: 640, height: 480},
			"tools": &testWindow{width: 320, height: 240},
		},
		projectionLayers: map[facet.FacetID]facet.ProjectionLayer{
			1: {LayerID: 10},
			2: {LayerID: 11},
		},
	}
	frameOut := &projection.FrameOutput{
		RenderBatchs: []projection.RenderBatchOutput{
			{FacetID: 1, Bounds: gfx.RectFromXYWH(0, 0, 10, 10), Commands: gfx.CommandList{}},
			{FacetID: 2, Bounds: gfx.RectFromXYWH(10, 10, 20, 20), Commands: gfx.CommandList{}},
		},
	}
	frames := rt.assembleWindowFrames(frameOut, nil)
	if len(frames) != 2 {
		t.Fatalf("frames = %#v", frames)
	}
	if got := frames[windowBindingKey(layout.WindowBinding{Kind: layout.WindowBindingPrimary})]; got == nil || len(got.RenderBatchs) != 1 {
		t.Fatalf("primary frame = %#v", got)
	}
	if got := frames["tools"]; got == nil || len(got.RenderBatchs) != 1 {
		t.Fatalf("tools frame = %#v", got)
	}
}
