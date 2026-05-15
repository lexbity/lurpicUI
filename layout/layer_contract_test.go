package layout

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
)

func TestLayerContract_zeroValues_areStable(t *testing.T) {
	if PlacementStack != 0 {
		t.Fatalf("PlacementStack = %d, want 0", PlacementStack)
	}
	if MeasureStructural != 0 {
		t.Fatalf("MeasureStructural = %d, want 0", MeasureStructural)
	}
	if CoordParentLayout != 0 {
		t.Fatalf("CoordParentLayout = %d, want 0", CoordParentLayout)
	}
	if HitNormal != 0 {
		t.Fatalf("HitNormal = %d, want 0", HitNormal)
	}
	if ClipNone != 0 {
		t.Fatalf("ClipNone = %d, want 0", ClipNone)
	}
}

func TestLayerContract_validate_requires_layer_identity(t *testing.T) {
	if err := ValidateLayerSpec(LayerSpec{}); err == nil {
		t.Fatal("expected zero-value LayerSpec to be rejected")
	}
}

func TestLayerContract_validate_accepts_explicit_layer_contract(t *testing.T) {
	spec := LayerSpec{
		ID:          1,
		Placement:   PlacementStack,
		Measurement: MeasureStructural,
		CoordSpace:  CoordParentLayout,
		CoordLimits: CoordLimits{Bounds: gfx.RectFromXYWH(0, 0, 64, 32)},
		HitPolicy:   HitNormal,
		RenderOrder: 0,
		ClipPolicy:  ClipNone,
	}
	if err := ValidateLayerSpec(spec); err != nil {
		t.Fatalf("expected valid spec, got %v", err)
	}
}
