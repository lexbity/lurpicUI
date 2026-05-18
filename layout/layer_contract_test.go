package layout

import "testing"

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
