package runtime

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/input"
	"codeburg.org/lexbit/lurpicui/projection"
)

func dismissalTestRT() *Runtime {
	return &Runtime{
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
}

func dismissalTestHitMap() *projection.HitMap {
	return projection.NewHitMap(
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
}

func pointerPress(target facet.FacetID, markID facet.MarkID) input.RoutedEvent {
	return input.RoutedEvent{
		Target: target,
		Event: input.PointerPressEvent{
			Position:  gfx.Point{X: 5, Y: 5},
			ScreenPos: gfx.Point{X: 5, Y: 5},
			MarkID:    markID,
		},
	}
}

func TestDismissalEvents_forPress_prepend_dismissals(t *testing.T) {
	rt := dismissalTestRT()
	hitMap := dismissalTestHitMap()

	routed := rt.withDismissalEvents([]input.RoutedEvent{pointerPress(2, 22)}, hitMap)
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

func TestDismissalEvents_nonPointerEvent_noDismissal(t *testing.T) {
	rt := dismissalTestRT()
	hitMap := dismissalTestHitMap()

	routed := rt.withDismissalEvents([]input.RoutedEvent{{
		Target: 2,
		Event:  input.FocusGainedEvent{},
	}}, hitMap)
	if len(routed) != 1 {
		t.Fatalf("len = %d, want 1 (no dismissal for non-pointer event)", len(routed))
	}
}

func TestDismissalEvents_outsideOrderRange_noDismissal(t *testing.T) {
	rt := dismissalTestRT()
	hitMap := dismissalTestHitMap()

	routed := rt.withDismissalEvents([]input.RoutedEvent{pointerPress(2, 22)}, hitMap)
	if len(routed) != 2 {
		t.Fatalf("len = %d, expected 1", len(routed))
	}

	rt2 := &Runtime{
		projectionLayers: map[facet.FacetID]facet.ProjectionLayer{
			1: {
				LayerID: 1,
				Dismissal: facet.DismissalScope{
					Enabled:      true,
					BehindOrders: facet.OrderRange{Min: 7000, Max: 8000},
					Triggers:     facet.DismissalTriggerSetPointer,
				},
			},
			2: {LayerID: 2},
		},
	}
	hm2 := projection.NewHitMap(
		projection.HitMapEntry{FacetID: 1, LayerID: 1, LayerOrder: 6000, Transform: gfx.Identity(),
			Regions: []projection.HitRegion{{Bounds: gfx.RectFromXYWH(0, 0, 10, 10), MarkID: 11}}},
		projection.HitMapEntry{FacetID: 2, LayerID: 2, LayerOrder: 1000, Transform: gfx.Identity(),
			Regions: []projection.HitRegion{{Bounds: gfx.RectFromXYWH(0, 0, 10, 10), MarkID: 22}}},
	)
	routed2 := rt2.withDismissalEvents([]input.RoutedEvent{pointerPress(2, 22)}, hm2)
	if len(routed2) != 1 {
		t.Fatalf("len = %d, want 1 (target order 1000 not in [7000, 8000])", len(routed2))
	}
}

func TestDismissalEvents_disabledScope_noDismissal(t *testing.T) {
	rt := &Runtime{
		projectionLayers: map[facet.FacetID]facet.ProjectionLayer{
			1: {
				LayerID: 1,
				Dismissal: facet.DismissalScope{
					Enabled:      false,
					BehindOrders: facet.OrderRange{Min: 0, Max: 5000},
					Triggers:     facet.DismissalTriggerSetPointer,
				},
			},
			2: {LayerID: 2},
		},
	}
	hitMap := dismissalTestHitMap()

	routed := rt.withDismissalEvents([]input.RoutedEvent{pointerPress(2, 22)}, hitMap)
	if len(routed) != 1 {
		t.Fatalf("len = %d, want 1 (dismissal disabled)", len(routed))
	}
}

func TestDismissalEvents_noDismissalScope_noPrepend(t *testing.T) {
	rt := &Runtime{
		projectionLayers: map[facet.FacetID]facet.ProjectionLayer{
			1: {LayerID: 1},
			2: {LayerID: 2},
		},
	}
	hitMap := dismissalTestHitMap()

	routed := rt.withDismissalEvents([]input.RoutedEvent{pointerPress(2, 22)}, hitMap)
	if len(routed) != 1 {
		t.Fatalf("len = %d, want 1 (no dismissal scope)", len(routed))
	}
}
