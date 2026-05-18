package runtime

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/input"
	"codeburg.org/lexbit/lurpicui/projection"
)

func TestDismissalEvents_forPress_prepend_dismissals(t *testing.T) {
	rt := &Runtime{
		projectionLayers: map[facet.FacetID]facet.ProjectionLayer{
			1: {
				LayerID: 1,
				Dismissal: facet.DismissalScope{
					Enabled:      true,
					BehindOrders: facet.OrderRange{Min: 0, Max: 5000},
					Triggers:     facet.DismissalTriggerSetPointer,
				},
			},
			2: {
				LayerID: 2,
			},
		},
	}
	hitMap := projection.NewHitMap(
		projection.HitMapEntry{
			FacetID:    1,
			LayerID:    1,
			LayerOrder: 6000,
			Transform:  gfx.Identity(),
			Regions:    []projection.HitRegion{{Bounds: gfx.RectFromXYWH(0, 0, 10, 10), MarkID: 11}},
		},
		projection.HitMapEntry{
			FacetID:    2,
			LayerID:    2,
			LayerOrder: 1000,
			Transform:  gfx.Identity(),
			Regions:    []projection.HitRegion{{Bounds: gfx.RectFromXYWH(0, 0, 10, 10), MarkID: 22}},
		},
	)
	routed := rt.withDismissalEvents([]input.RoutedEvent{{
		Target: 2,
		Event: input.PointerPressEvent{
			Position:  gfx.Point{X: 5, Y: 5},
			ScreenPos: gfx.Point{X: 5, Y: 5},
			MarkID:    22,
		},
	}}, hitMap)
	if len(routed) != 2 {
		t.Fatalf("len = %d", len(routed))
	}
	dismiss, ok := routed[0].Event.(input.DismissEvent)
	if !ok {
		t.Fatalf("first event = %#v", routed[0].Event)
	}
	if routed[0].Target != 1 || dismiss.HitFacetID != 2 || dismiss.HitMarkID != 22 || dismiss.HitOrder != 1000 {
		t.Fatalf("dismiss = %#v target=%d", dismiss, routed[0].Target)
	}
	if _, ok := routed[1].Event.(input.PointerPressEvent); !ok {
		t.Fatalf("second event = %#v", routed[1].Event)
	}
}
