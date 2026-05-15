package structure

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks/basic"
)

func TestLayerMount_contract_mounts_child_into_target_layer_only(t *testing.T) {
	child := &basic.Rect{
		Bounds: basic.BoundsProps{X: 0, Y: 0, W: 10, H: 10},
		Style:  basic.PrimitiveStyleProps{Visible: true, Opacity: 1},
	}
	root := &LayerMount{
		TargetLayer: 7,
		Child:       child,
	}

	specs := root.OnLayerSpecs()
	if len(specs) != 1 {
		t.Fatalf("LayerSpecs = %d, want 1", len(specs))
	}
	if specs[0].ID != 7 || specs[0].RenderOrder != 7 {
		t.Fatalf("LayerSpec = %#v, want target layer 7", specs[0])
	}

	attachStructureTree(t, root)
	hostBounds := gfx.RectFromXYWH(24, 36, 120, 80)
	root.Base().LayoutRole().Arrange(hostBounds)

	if got := child.Base().LayoutRole().ArrangedBounds; got != hostBounds {
		t.Fatalf("child arranged bounds = %#v, want %#v", got, hostBounds)
	}
}
