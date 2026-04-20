package layout

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
)

func TestValidateLayerSpec_accepts_valid_specs(t *testing.T) {
	cases := []LayerSpec{
		{ID: 1, Placement: PlacementStack, Measurement: MeasureStructural},
		{ID: 2, Placement: PlacementSplit, Measurement: MeasureStructural},
		{ID: 3, Placement: PlacementGrid, Measurement: MeasureStructural},
		{ID: 4, Placement: PlacementFree, Measurement: MeasureNonStructural},
		{ID: 5, Placement: PlacementAnchor, Measurement: MeasureNonStructural},
		{ID: 6, Placement: PlacementProjected, Measurement: MeasureNonStructural},
	}
	for _, spec := range cases {
		if err := ValidateLayerSpec(spec); err != nil {
			t.Fatalf("expected valid spec for %+v, got %v", spec, err)
		}
	}
}

func TestValidateLayerSpec_rejects_invalid_contracts(t *testing.T) {
	cases := []struct {
		name string
		spec LayerSpec
	}{
		{
			name: "zero id",
			spec: LayerSpec{Placement: PlacementStack, Measurement: MeasureStructural},
		},
		{
			name: "anchor structural",
			spec: LayerSpec{ID: 1, Placement: PlacementAnchor, Measurement: MeasureStructural},
		},
		{
			name: "anchor hybrid",
			spec: LayerSpec{ID: 1, Placement: PlacementAnchor, Measurement: MeasureHybrid},
		},
		{
			name: "projected structural",
			spec: LayerSpec{ID: 1, Placement: PlacementProjected, Measurement: MeasureStructural},
		},
		{
			name: "projected hybrid",
			spec: LayerSpec{ID: 1, Placement: PlacementProjected, Measurement: MeasureHybrid},
		},
		{
			name: "negative render order",
			spec: LayerSpec{ID: 1, Placement: PlacementStack, Measurement: MeasureStructural, RenderOrder: -1},
		},
	}
	for _, tc := range cases {
		if err := ValidateLayerSpec(tc.spec); err == nil {
			t.Fatalf("expected error for %s", tc.name)
		}
	}
}

func TestDiffLayerSpecs_identical_slices(t *testing.T) {
	oldSpecs := []LayerSpec{
		{ID: 1, Placement: PlacementStack, Measurement: MeasureStructural, HitPolicy: HitNormal, RenderOrder: 1},
	}
	newSpecs := []LayerSpec{
		{ID: 1, Placement: PlacementStack, Measurement: MeasureStructural, HitPolicy: HitNormal, RenderOrder: 1},
	}
	if diff := DiffLayerSpecs(oldSpecs, newSpecs); diff != (LayerDiff{}) {
		t.Fatalf("unexpected diff: %#v", diff)
	}
}

func TestDiffLayerSpecs_hit_policy_only(t *testing.T) {
	oldSpecs := []LayerSpec{{ID: 1, Placement: PlacementStack, Measurement: MeasureStructural, HitPolicy: HitNormal}}
	newSpecs := []LayerSpec{{ID: 1, Placement: PlacementStack, Measurement: MeasureStructural, HitPolicy: HitPassThrough}}
	if diff := DiffLayerSpecs(oldSpecs, newSpecs); diff != (LayerDiff{NeedsProjection: true}) {
		t.Fatalf("unexpected diff: %#v", diff)
	}
}

func TestDiffLayerSpecs_render_order_only(t *testing.T) {
	oldSpecs := []LayerSpec{{ID: 1, Placement: PlacementStack, Measurement: MeasureStructural, RenderOrder: 0}}
	newSpecs := []LayerSpec{{ID: 1, Placement: PlacementStack, Measurement: MeasureStructural, RenderOrder: 3}}
	if diff := DiffLayerSpecs(oldSpecs, newSpecs); diff != (LayerDiff{NeedsProjection: true}) {
		t.Fatalf("unexpected diff: %#v", diff)
	}
}

func TestDiffLayerSpecs_placement_change_requires_both(t *testing.T) {
	oldSpecs := []LayerSpec{{ID: 1, Placement: PlacementStack, Measurement: MeasureStructural}}
	newSpecs := []LayerSpec{{ID: 1, Placement: PlacementGrid, Measurement: MeasureStructural}}
	if diff := DiffLayerSpecs(oldSpecs, newSpecs); diff != (LayerDiff{NeedsLayout: true, NeedsProjection: true}) {
		t.Fatalf("unexpected diff: %#v", diff)
	}
}

func TestDiffLayerSpecs_coordspace_change_requires_both(t *testing.T) {
	oldSpecs := []LayerSpec{{ID: 1, Placement: PlacementStack, Measurement: MeasureStructural, CoordSpace: CoordParentLayout}}
	newSpecs := []LayerSpec{{ID: 1, Placement: PlacementStack, Measurement: MeasureStructural, CoordSpace: CoordViewport}}
	if diff := DiffLayerSpecs(oldSpecs, newSpecs); diff != (LayerDiff{NeedsLayout: true, NeedsProjection: true}) {
		t.Fatalf("unexpected diff: %#v", diff)
	}
}

func TestDiffLayerSpecs_measurement_change_requires_both(t *testing.T) {
	oldSpecs := []LayerSpec{{ID: 1, Placement: PlacementStack, Measurement: MeasureStructural}}
	newSpecs := []LayerSpec{{ID: 1, Placement: PlacementStack, Measurement: MeasureNonStructural}}
	if diff := DiffLayerSpecs(oldSpecs, newSpecs); diff != (LayerDiff{NeedsLayout: true, NeedsProjection: true}) {
		t.Fatalf("unexpected diff: %#v", diff)
	}
}

func TestDiffLayerSpecs_nonstructural_coordlimits_bounds_projection_only(t *testing.T) {
	oldSpecs := []LayerSpec{{ID: 1, Placement: PlacementFree, Measurement: MeasureNonStructural, CoordLimits: CoordLimits{Bounds: gfx.RectFromXYWH(0, 0, 10, 10)}}}
	newSpecs := []LayerSpec{{ID: 1, Placement: PlacementFree, Measurement: MeasureNonStructural, CoordLimits: CoordLimits{Bounds: gfx.RectFromXYWH(0, 0, 20, 20)}}}
	if diff := DiffLayerSpecs(oldSpecs, newSpecs); diff != (LayerDiff{NeedsProjection: true}) {
		t.Fatalf("unexpected diff: %#v", diff)
	}
}

func TestDiffLayerSpecs_structural_coordlimits_bounds_requires_both(t *testing.T) {
	oldSpecs := []LayerSpec{{ID: 1, Placement: PlacementStack, Measurement: MeasureStructural, CoordLimits: CoordLimits{Bounds: gfx.RectFromXYWH(0, 0, 10, 10)}}}
	newSpecs := []LayerSpec{{ID: 1, Placement: PlacementStack, Measurement: MeasureStructural, CoordLimits: CoordLimits{Bounds: gfx.RectFromXYWH(0, 0, 20, 20)}}}
	if diff := DiffLayerSpecs(oldSpecs, newSpecs); diff != (LayerDiff{NeedsLayout: true, NeedsProjection: true}) {
		t.Fatalf("unexpected diff: %#v", diff)
	}
}

func TestDiffLayerSpecs_length_change_requires_both(t *testing.T) {
	oldSpecs := []LayerSpec{{ID: 1, Placement: PlacementStack, Measurement: MeasureStructural}}
	newSpecs := []LayerSpec{
		{ID: 1, Placement: PlacementStack, Measurement: MeasureStructural},
		{ID: 2, Placement: PlacementStack, Measurement: MeasureStructural},
	}
	if diff := DiffLayerSpecs(oldSpecs, newSpecs); diff != (LayerDiff{NeedsLayout: true, NeedsProjection: true}) {
		t.Fatalf("unexpected diff: %#v", diff)
	}
}

func TestAnchorPositionCache_update_get_version(t *testing.T) {
	cache := NewAnchorPositionCache()
	if got := cache.Version(); got != 0 {
		t.Fatalf("unexpected initial version: %d", got)
	}
	if changed := cache.Update("a", gfx.Point{X: 1, Y: 2}); !changed {
		t.Fatal("expected first update to change cache")
	}
	if got := cache.Version(); got != 1 {
		t.Fatalf("unexpected version after first update: %d", got)
	}
	if changed := cache.Update("a", gfx.Point{X: 1, Y: 2}); changed {
		t.Fatal("expected identical update to be ignored")
	}
	if got := cache.Version(); got != 1 {
		t.Fatalf("unexpected version after identical update: %d", got)
	}
	if changed := cache.Update("a", gfx.Point{X: 3, Y: 4}); !changed {
		t.Fatal("expected moved position to change cache")
	}
	if got := cache.Version(); got != 2 {
		t.Fatalf("unexpected version after move: %d", got)
	}
	pos, ok := cache.Get("a")
	if !ok || pos != (gfx.Point{X: 3, Y: 4}) {
		t.Fatalf("unexpected cache value: pos=%#v ok=%v", pos, ok)
	}
	if _, ok := cache.Get("missing"); ok {
		t.Fatal("expected missing anchor to be absent")
	}
}

func TestPlacementHints_zero_value_is_valid(t *testing.T) {
	var hints PlacementHints
	if hints.Align != AlignStretch {
		t.Fatalf("unexpected zero alignment: %v", hints.Align)
	}
	if hints.FreeAnchor != FreeTopLeft {
		t.Fatalf("unexpected zero free anchor: %v", hints.FreeAnchor)
	}
	if hints.AnchorSide != AnchorAbove {
		t.Fatalf("unexpected zero anchor side: %v", hints.AnchorSide)
	}
}
