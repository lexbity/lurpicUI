package layout

import (
	"math"
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

type layerPolicyChildFixture struct {
	child *LayerChild
	role  *facet.LayoutRole
}

func newLayerPolicyChildFixture(size gfx.Size, mode facet.PlacementMode, supported facet.PlacementModeSet) layerPolicyChildFixture {
	base := facet.NewFacet()
	role := &facet.LayoutRole{}
	role.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: size}
	}
	role.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		role.ArrangedBounds = bounds
	}
	role.Child.SupportedPlacement = supported
	attachment := facet.Attachment{
		Placement: facet.Placement{Mode: mode},
	}
	return layerPolicyChildFixture{
		child: &LayerChild{
			FacetID:    base.ID(),
			Attachment: attachment,
			Layout:     role,
			Descriptor: role.Child,
		},
		role: role,
	}
}

func TestDefaultLayerLayoutPolicy_usesFiveByFiveGrid(t *testing.T) {
	policy := ResolveLayerLayoutPolicy(ResolvedLayerLayoutRecipe{})
	if policy.Kind() != LayerLayoutGrid {
		t.Fatalf("policy kind = %v, want grid", policy.Kind())
	}
	child := newLayerPolicyChildFixture(gfx.Size{W: 20, H: 10}, facet.PlacementGrid, facet.SupportsGrid)
	measureCtx := LayerMeasureContext{
		Layer:  facet.LayerContext{ID: facet.LayerID(1)},
		Bounds: gfx.RectFromXYWH(0, 0, 500, 500),
		Recipe: DefaultLayerLayoutRecipe(),
	}
	arrangeCtx := LayerArrangeContext{LayerMeasureContext: measureCtx}
	measure, err := policy.MeasureLayer(measureCtx, []LayerChild{*child.child})
	if err != nil {
		t.Fatalf("MeasureLayer: %v", err)
	}
	if measure.Size != (gfx.Size{W: 500, H: 500}) {
		t.Fatalf("measure = %#v, want 500x500", measure.Size)
	}
	arranged, err := policy.ArrangeLayer(arrangeCtx, []LayerChild{*child.child})
	if err != nil {
		t.Fatalf("ArrangeLayer: %v", err)
	}
	if len(arranged) != 1 {
		t.Fatalf("arranged count = %d, want 1", len(arranged))
	}
	if arranged[0].Bounds != (gfx.RectFromXYWH(0, 0, 100, 100)) {
		t.Fatalf("arranged bounds = %#v, want first 5x5 cell", arranged[0].Bounds)
	}
	if child.role.ArrangedBounds != arranged[0].Bounds {
		t.Fatalf("role arranged bounds = %#v, want %#v", child.role.ArrangedBounds, arranged[0].Bounds)
	}
}

func TestLayerLayoutPolicy_gridRejectsInvalidSpan(t *testing.T) {
	policy := ResolveLayerLayoutPolicy(DefaultLayerLayoutRecipe())
	child := newLayerPolicyChildFixture(gfx.Size{W: 20, H: 10}, facet.PlacementGrid, facet.SupportsGrid)
	child.child.Attachment.Placement.Grid = facet.GridPlacement{ColStart: 0, RowStart: 0, ColSpan: 0, RowSpan: 1}
	ctx := LayerArrangeContext{
		LayerMeasureContext: LayerMeasureContext{
			Layer:  facet.LayerContext{ID: facet.LayerID(1)},
			Bounds: gfx.RectFromXYWH(0, 0, 500, 500),
			Recipe: DefaultLayerLayoutRecipe(),
		},
	}
	_, err := policy.ArrangeLayer(ctx, []LayerChild{*child.child})
	if err == nil || !strings.Contains(err.Error(), "grid span") {
		t.Fatalf("ArrangeLayer error = %v, want grid span validation", err)
	}
}

func TestLayerLayoutPolicy_anchorResolvesExistingAnchor(t *testing.T) {
	policy := ResolveLayerLayoutPolicy(ResolvedLayerLayoutRecipe{PolicyKind: LayerLayoutAnchor})
	child := newLayerPolicyChildFixture(gfx.Size{W: 20, H: 10}, facet.PlacementAnchor, facet.SupportsAnchor)
	child.child.Attachment.Placement.Anchor = facet.AnchorPlacement{
		AnchorRef: facet.AnchorID("mark"),
		Side:      facet.AnchorRight,
		Gap:       facet.ResolvedScalar(5),
	}
	cache := NewAnchorPositionCache()
	cache.Update(AnchorID("mark"), gfx.Point{X: 10, Y: 20})
	ctx := LayerArrangeContext{
		LayerMeasureContext: LayerMeasureContext{
			Layer:       facet.LayerContext{ID: facet.LayerID(1)},
			Bounds:      gfx.RectFromXYWH(0, 0, 500, 500),
			Recipe:      ResolvedLayerLayoutRecipe{PolicyKind: LayerLayoutAnchor},
			AnchorCache: cache,
		},
	}
	arranged, err := policy.ArrangeLayer(ctx, []LayerChild{*child.child})
	if err != nil {
		t.Fatalf("ArrangeLayer: %v", err)
	}
	if arranged[0].Bounds != (gfx.RectFromXYWH(15, 15, 20, 10)) {
		t.Fatalf("arranged bounds = %#v, want anchored rect", arranged[0].Bounds)
	}
}

func TestLayerLayoutPolicy_anchorRejectsMissingAnchor(t *testing.T) {
	policy := ResolveLayerLayoutPolicy(ResolvedLayerLayoutRecipe{PolicyKind: LayerLayoutAnchor})
	child := newLayerPolicyChildFixture(gfx.Size{W: 20, H: 10}, facet.PlacementAnchor, facet.SupportsAnchor)
	child.child.Attachment.Placement.Anchor = facet.AnchorPlacement{
		AnchorRef: facet.AnchorID("missing"),
		Side:      facet.AnchorRight,
	}
	cache := NewAnchorPositionCache()
	ctx := LayerArrangeContext{
		LayerMeasureContext: LayerMeasureContext{
			Layer:       facet.LayerContext{ID: facet.LayerID(1)},
			Bounds:      gfx.RectFromXYWH(0, 0, 500, 500),
			Recipe:      ResolvedLayerLayoutRecipe{PolicyKind: LayerLayoutAnchor},
			AnchorCache: cache,
		},
	}
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for missing anchor")
		} else if msg, ok := r.(string); !ok || !strings.Contains(msg, "layout contract violation") || !strings.Contains(msg, "anchor \"missing\"") {
			t.Fatalf("panic = %#v", r)
		}
	}()
	_, _ = policy.ArrangeLayer(ctx, []LayerChild{*child.child})
}

func TestLayerLayoutPolicy_freePlacementUsesMeasuredSizeWhenUnspecified(t *testing.T) {
	policy := ResolveLayerLayoutPolicy(ResolvedLayerLayoutRecipe{PolicyKind: LayerLayoutFree})
	child := newLayerPolicyChildFixture(gfx.Size{W: 30, H: 40}, facet.PlacementFree, facet.SupportsFree)
	child.child.Attachment.Placement.Free = facet.FreePlacement{
		X: facet.ResolvedScalar(12),
		Y: facet.ResolvedScalar(34),
	}
	ctx := LayerArrangeContext{
		LayerMeasureContext: LayerMeasureContext{
			Layer:  facet.LayerContext{ID: facet.LayerID(1)},
			Bounds: gfx.RectFromXYWH(0, 0, 500, 500),
			Recipe: ResolvedLayerLayoutRecipe{PolicyKind: LayerLayoutFree},
		},
	}
	arranged, err := policy.ArrangeLayer(ctx, []LayerChild{*child.child})
	if err != nil {
		t.Fatalf("ArrangeLayer: %v", err)
	}
	if arranged[0].Bounds != (gfx.RectFromXYWH(12, 34, 30, 40)) {
		t.Fatalf("arranged bounds = %#v, want free-placement rect", arranged[0].Bounds)
	}
}

func TestLayerLayoutPolicy_freeRejectsNonFiniteCoordinates(t *testing.T) {
	policy := ResolveLayerLayoutPolicy(ResolvedLayerLayoutRecipe{PolicyKind: LayerLayoutFree})
	child := newLayerPolicyChildFixture(gfx.Size{W: 30, H: 40}, facet.PlacementFree, facet.SupportsFree)
	child.child.Attachment.Placement.Free = facet.FreePlacement{
		X: facet.ResolvedScalar(float32(math.NaN())),
		Y: facet.ResolvedScalar(34),
	}
	ctx := LayerArrangeContext{
		LayerMeasureContext: LayerMeasureContext{
			Layer:  facet.LayerContext{ID: facet.LayerID(1)},
			Bounds: gfx.RectFromXYWH(0, 0, 500, 500),
			Recipe: ResolvedLayerLayoutRecipe{PolicyKind: LayerLayoutFree},
		},
	}
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for non-finite free placement coordinate")
		}
	}()
	_, _ = policy.ArrangeLayer(ctx, []LayerChild{*child.child})
}
