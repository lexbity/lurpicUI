package feedback

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/layout"
)

// overlayBackground is the distinct color the overlay test root fills the
// window with. The overlay surface renders on top of it, so the overlay's
// region is pixel-distinguishable before and after dismissal.
var overlayBackground = gfx.Color{R: 0.2, G: 0.4, B: 0.8, A: 1}

// overlayRootFacet is a full-window, hit-testable background for overlay tests.
// It is hit everywhere, so a press outside the overlay's bounds lands on this
// base-layer facet, which is what routes the outside click through the runtime's
// dismissalEventsForPointerPresses to the overlay's OnDismiss.
type overlayRootFacet struct {
	facet.Facet
	layout facet.LayoutRole
	hit    facet.HitRole
	render facet.RenderRole
}

func newOverlayRoot() *overlayRootFacet {
	f := &overlayRootFacet{Facet: facet.NewFacet()}
	f.hit.OnHitTest = func(p gfx.Point) facet.HitResult {
		return facet.HitResult{Hit: true, MarkID: 1}
	}
	f.layout.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: c.MaxSize}
	}
	f.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		f.layout.ArrangedBounds = bounds
	}
	f.layout.Child.SupportedPlacement = facet.SupportsGrid | facet.SupportsAnchor | facet.SupportsFree | facet.SupportsLinear
	f.render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(overlayBackground)})
	}
	f.AddRole(&f.layout)
	f.AddRole(&f.hit)
	f.AddRole(&f.render)
	return f
}

func (f *overlayRootFacet) Base() *facet.Facet               { return &f.Facet }
func (f *overlayRootFacet) OnAttach(ctx facet.AttachContext) {}
func (f *overlayRootFacet) OnDetach()                        {}
func (f *overlayRootFacet) OnActivate()                      {}
func (f *overlayRootFacet) OnDeactivate()                    {}

// dismissalRegistry returns a layer registry whose "app.modal" layer enables
// outside-click (pointer) dismissal of the layers behind it. The standard
// layers ship without a Dismissal scope, so an overlay must be mounted into a
// dismissal-enabled layer for the runtime's dismissalEventsForPointerPresses to
// emit DismissEvents.
func dismissalRegistry(t *testing.T) *layout.LayerRegistry {
	t.Helper()
	b := layout.NewLayerRegistryBuilder()
	if err := b.RegisterStandardLayers(); err != nil {
		t.Fatalf("register standard layers: %v", err)
	}
	if _, err := b.RegisterLayer(layout.LayerRegistration{
		Name:  "app.modal",
		Order: 7500,
		Dismissal: layout.DismissalScope{
			Enabled:      true,
			BehindOrders: layout.OrderRange{Min: 0, Max: 8000},
			Triggers:     layout.DismissalTriggerSetPointer,
		},
	}); err != nil {
		t.Fatalf("register app.modal layer: %v", err)
	}
	reg, err := b.Freeze()
	if err != nil {
		t.Fatalf("freeze registry: %v", err)
	}
	return reg
}

// newOverlayHarness builds a harness over a hit-testable root using the
// dismissal-enabled registry, returning the harness and the modal layer's ID.
// Overlays are mounted on the root with facet.AttachLayer before this call and
// bound to the modal layer with UpdateChildAttachment afterwards.
func newOverlayHarness(t *testing.T, root facet.FacetImpl) (*testkit.Harness, facet.LayerID) {
	t.Helper()
	reg := dismissalRegistry(t)
	desc, ok := reg.LookupName("app.modal")
	if !ok {
		t.Fatal("missing app.modal layer")
	}
	cfg := testkit.StandardHarnessConfig(t, 320, 200)
	cfg.LayerRegistry = reg
	h := testkit.NewHarness(t, cfg, root)
	return h, facet.LayerID(desc.ID)
}
