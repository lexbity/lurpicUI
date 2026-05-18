package runtime

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/layout"
)

func TestRuntime_syncFocusTraps_restoresPreviousFocus(t *testing.T) {
	builder := layout.NewLayerRegistryBuilder()
	if err := builder.RegisterStandardLayers(); err != nil {
		t.Fatalf("register standard layers: %v", err)
	}
	trapLayerID, err := builder.RegisterLayer(layout.LayerRegistration{
		Name:          "trap",
		Order:         2500,
		WindowBinding: layout.WindowBinding{Kind: layout.WindowBindingPrimary},
		FocusTrap:     true,
		FocusRestore:  facet.FocusRestorePrevious,
	})
	if err != nil {
		t.Fatalf("register trap layer: %v", err)
	}
	reg, err := builder.Freeze()
	if err != nil {
		t.Fatalf("freeze registry: %v", err)
	}

	root := newRuntimeFocusFacet(0)
	outside := newRuntimeFocusFacet(0)
	trap := newRuntimeFocusFacet(10)
	trap.focus.Focusable = func() bool { return false }
	inner := newRuntimeFocusFacet(0)
	root.Base().AddChildRuntime(outside.Base())
	root.Base().AddChildRuntime(trap.Base())
	trap.Base().AddChildRuntime(inner.Base())

	rt := &Runtime{
		focusManager:     facet.NewFocusManager(),
		layerRegistry:    reg,
		projectionLayers: make(map[facet.FacetID]facet.ProjectionLayer),
	}
	rt.projectionLayers[trap.ID()] = facet.ProjectionLayer{LayerID: facet.LayerID(trapLayerID)}
	rt.focusManager.RebuildTabOrder(root)
	if !rt.focusManager.SetFocus(outside) {
		t.Fatal("set outside focus")
	}

	rt.syncFocusTraps()
	if got := rt.focusManager.Focused(); got != inner.ID() {
		t.Fatalf("focused = %d, want %d", got, inner.ID())
	}

	delete(rt.projectionLayers, trap.ID())
	rt.syncFocusTraps()
	if got := rt.focusManager.Focused(); got != outside.ID() {
		t.Fatalf("restored focus = %d, want %d", got, outside.ID())
	}
}
