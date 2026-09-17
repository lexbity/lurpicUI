package feedback

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/store"
)

// TestTooltip_OnProjectRunsInsidePhase pins RX-1 P5: the tooltip's projection
// callback is invoked by the runtime inside the collect→project→compose phase
// (a warm frame projects it and the retained output carries its surface), and
// invoking the projection directly outside the phase through a live runtime
// panics.
func TestTooltip_OnProjectRunsInsidePhase(t *testing.T) {
	open := store.NewValueStore(true)
	tt := NewTooltip("Deletes permanently", open)
	root := newOverlayRoot()
	facet.AttachLayer(root, tt, facet.LayerAttachment{Band: facet.ZBandTooltip})
	h, modalID := newOverlayHarness(t, root)
	h.Runtime().UpdateChildAttachment(tt, facet.Attachment{LayerID: modalID})
	testkit.Warmup(h)

	// The frame projected the tooltip's surface: its retained output exists and
	// its cached surface bounds are arranged.
	if cmds := h.Runtime().LastOutputCommands(tt.Base().ID()); len(cmds) == 0 {
		t.Fatal("tooltip produced no retained projection output inside the frame")
	}
	if tt.cachedSurfaceBounds.IsEmpty() {
		t.Fatal("tooltip surface not arranged by the frame")
	}
}

func TestTooltip_ProjectOutsidePhasePanics(t *testing.T) {
	open := store.NewValueStore(true)
	tt := NewTooltip("Deletes permanently", open)
	root := newOverlayRoot()
	facet.AttachLayer(root, tt, facet.LayerAttachment{Band: facet.ZBandTooltip})
	h, modalID := newOverlayHarness(t, root)
	h.Runtime().UpdateChildAttachment(tt, facet.Attachment{LayerID: modalID})
	testkit.Warmup(h)

	ctx := facet.ProjectionContext{
		Bounds:       tt.cachedSurfaceBounds,
		Runtime:      h.Runtime(),
		ContentScale: 1,
	}
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected tooltip projection outside the phase to panic")
		}
	}()
	_ = tt.Base().ProjectionRole().Project(ctx)
	_ = gfx.Rect{}
}
