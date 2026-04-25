package interaction

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
)

func TestHoverState(t *testing.T) {
	t.Parallel()

	var hovered, pressed bool
	if !HoverState(&hovered, &pressed, false, platform.PointerEnter, true) {
		t.Fatal("expected enter to be handled")
	}
	if !hovered {
		t.Fatal("expected hovered=true after enter")
	}
	if !HoverState(&hovered, &pressed, false, platform.PointerLeave, true) {
		t.Fatal("expected leave to be handled")
	}
	if hovered || pressed {
		t.Fatalf("expected hovered/pressed to clear, got %v/%v", hovered, pressed)
	}
}

func TestPressReleaseState(t *testing.T) {
	t.Parallel()

	var pressed bool
	var activated bool
	if !PressReleaseState(&pressed, false, facet.PointerEvent{Kind: platform.PointerPress}, func() { activated = true }) {
		t.Fatal("expected press to be handled")
	}
	if activated {
		t.Fatal("activation should not happen on press")
	}
	if !PressReleaseState(&pressed, false, facet.PointerEvent{Kind: platform.PointerRelease}, func() { activated = true }) {
		t.Fatal("expected release to be handled")
	}
	if !activated {
		t.Fatal("activation should happen on release")
	}
}

func TestTouchTargetRect(t *testing.T) {
	t.Parallel()

	bounds := gfx.RectFromXYWH(10, 20, 20, 20)
	got := TouchTargetRect(bounds, 44)
	if got.Width() < 44 || got.Height() < 44 {
		t.Fatalf("TouchTargetRect() = %#v, want at least 44x44", got)
	}
}
