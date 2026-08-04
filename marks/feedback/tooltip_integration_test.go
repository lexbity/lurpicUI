package feedback

import (
	"image/color"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/store"
)

// The integration test proves the outside-click dismissal junction for the
// Tooltip mark through the real runtime path. The tooltip's own pointer handler
// ignores presses outside its bounds, so the dismissal is exclusively driven by
// the runtime's dismissalEventsForPointerPresses emitting a DismissEvent to the
// dismissal-enabled layer the tooltip is mounted in.
//
// Mounting: the tooltip is mounted as a layered overlay via facet.AttachLayer
// and bound to the registry's dismissal-enabled layer at runtime.

func TestTooltipIntegration_ClickOutsideDismisses(t *testing.T) {
	open := store.NewValueStore(true)
	tt := NewTooltip("Deletes permanently", open)

	root := newOverlayRoot()
	facet.AttachLayer(root, tt, facet.LayerAttachment{ZPriority: 90})

	h, modalID := newOverlayHarness(t, root)
	h.Runtime().UpdateChildAttachment(tt, facet.Attachment{LayerID: modalID})
	testkit.Warmup(h)

	sb := tt.cachedSurfaceBounds
	if sb.IsEmpty() {
		t.Fatal("expected the tooltip surface to be arranged after warmup")
	}
	px := int(sb.Min.X + sb.Width()/2)
	py := int(sb.Min.Y + sb.Height()/2)
	before := h.Surface().PixelAt(px, py)

	// Click far outside the tooltip's bounds; the tooltip's own handler is not
	// involved, so a close here can only come from the runtime dismissal path.
	testkit.DriveClick(h, 300, 180)

	if open.Get() {
		t.Fatal("expected the outside click to dismiss the tooltip")
	}
	after := h.Surface().PixelAt(px, py)
	if before == after {
		t.Fatalf("expected the dismissed tooltip's surface to leave the frame: before=%#v after=%#v", before, after)
	}
	// The overlay's render commands are gone: the surface region shows the
	// root background again.
	testkit.AssertPixelColor(t, h.Surface(), px, py, color.RGBA{R: 51, G: 102, B: 204, A: 255}, 2)
}
