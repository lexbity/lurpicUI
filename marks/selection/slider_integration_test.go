package selection

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/store"
)

// The integration tests prove the drag/pointer and keyboard junctions for the
// Slider mark through the runtime (Q7 path 1: mark-as-root). The store-mutation
// assertion is load-bearing; the drag maps the pointer x across the track.

func TestSliderIntegration_DragMutatesStore(t *testing.T) {
	value := store.NewValueStore(20.0)
	sl := NewSlider("Volume", 0, 100, 1, value)

	h := testkit.NewStandardHarness(t, 400, 80, sl)
	testkit.Warmup(h)

	thumb := sl.cachedThumbBounds
	if thumb.IsEmpty() {
		t.Fatal("expected thumb bounds after warmup")
	}
	track := sl.cachedTrackBounds
	if track.IsEmpty() {
		t.Fatal("expected track bounds after warmup")
	}

	// Drag from the current knob position to the right end of the track.
	startX := (thumb.Min.X + thumb.Max.X) / 2
	startY := (thumb.Min.Y + thumb.Max.Y) / 2
	endX := track.Max.X

	before := value.Get()
	if before != 20 {
		t.Fatalf("expected initial value 20, got %v", before)
	}

	testkit.DriveDrag(h, startX, startY, endX, startY)

	after := value.Get()
	if after <= before {
		t.Fatalf("expected a drag to the track end to raise the value: before=%v after=%v", before, after)
	}
	if after < 0 || after > 100 {
		t.Fatalf("expected the value to stay clamped to [0,100], got %v", after)
	}
}

func TestSliderIntegration_KeyboardAdjustsStore(t *testing.T) {
	value := store.NewValueStore(50.0)
	sl := NewSlider("Volume", 0, 100, 10, value)

	h := testkit.NewStandardHarness(t, 400, 80, sl)
	rt := h.Runtime()
	rt.SetFocus(sl)
	h.RunFrame()

	testkit.DriveKeyPress(h, platform.KeyRight, 0)
	if got := value.Get(); got != 60 {
		t.Fatalf("expected KeyRight to step the value to 60, got %v", got)
	}

	testkit.DriveKeyPress(h, platform.KeyHome, 0)
	if got := value.Get(); got != 0 {
		t.Fatalf("expected KeyHome to move the value to the minimum 0, got %v", got)
	}
}
