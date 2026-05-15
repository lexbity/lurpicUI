package facet

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
)

func TestViewportRole_local_transform_stays_layer_local(t *testing.T) {
	role := &ViewportRole{}
	role.SetPanZoom(gfx.Point{X: 18, Y: -6}, 3)

	local := gfx.Point{X: 2, Y: 4}
	layer := role.LocalToLayer(local)
	if layer != (gfx.Point{X: 24, Y: 6}) {
		t.Fatalf("LocalToLayer = %#v, want translated/scaled point", layer)
	}
	roundTrip, ok := role.LayerToLocal(layer)
	if !ok {
		t.Fatal("expected invertible local transform")
	}
	if roundTrip != local {
		t.Fatalf("LayerToLocal = %#v, want %#v", roundTrip, local)
	}
}
