package testkit

import (
	"codeburg.org/lexbit/lurpicui/platform"
)

// The runtime processes input events against the PREVIOUS frame's hit map
// (runtime/frame.go): platform events are drained and routed at the start of
// the frame loop, before layout and projection rebuild the hit map for the
// next frame. On the very first frame after NewHarness the hit map is nil
// (projection.System.currentHitMap is only assigned inside a run), so pointer
// events injected before any RunFrame are silently dropped — the classic
// "click did not fire" test bug.
//
// The Drive* helpers below are the canonical entry point for interaction
// tests: each runs a warmup frame first (so the hit map exists), injects its
// event(s), and runs a frame after each so the runtime processes them. Tests
// that inject raw events via InjectEvent MUST call Warmup (or RunFrame) first.

// Warmup runs a single frame if the harness has not yet run one, so that
// projection builds the hit map. It is a no-op once the harness has run a
// frame. Drive* helpers call it automatically; raw InjectEvent users must call
// it (or RunFrame) before injecting pointer events.
func Warmup(h *Harness) {
	if h == nil {
		return
	}
	if h.FrameCount == 0 {
		h.RunFrame()
	}
}

// DriveClick routes a left-button press+release click at (x,y) through the
// runtime, running a warmup frame first and a frame after each of the press
// and release. The release is what fires activation-style handlers.
func DriveClick(h *Harness, x, y float32) {
	if h == nil {
		return
	}
	Warmup(h)
	h.InjectEvent(PointerPress(x, y, platform.PointerLeft))
	h.RunFrame()
	h.InjectEvent(PointerRelease(x, y, platform.PointerLeft))
	h.RunFrame()
}

// DriveKeyPress routes a single key press through the runtime.
func DriveKeyPress(h *Harness, key platform.Key, mods platform.ModifierKeys) {
	if h == nil {
		return
	}
	Warmup(h)
	h.InjectEvent(KeyPress(key, mods))
	h.RunFrame()
}

// DriveKeyRelease routes a single key release through the runtime.
func DriveKeyRelease(h *Harness, key platform.Key, mods platform.ModifierKeys) {
	if h == nil {
		return
	}
	Warmup(h)
	h.InjectEvent(KeyRelease(key, mods))
	h.RunFrame()
}

// DriveType injects a sequence of text events.
func DriveType(h *Harness, s string) {
	if h == nil {
		return
	}
	Warmup(h)
	h.InjectEvent(TypeText(s))
	h.RunFrame()
}

// DriveDrag routes a press at (fromX,fromY), interpolated moves to (toX,toY),
// and a release, running a frame after each event so hit-testing and hover
// track the moving pointer.
func DriveDrag(h *Harness, fromX, fromY, toX, toY float32) {
	if h == nil {
		return
	}
	Warmup(h)
	for _, e := range Drag(fromX, fromY, toX, toY) {
		h.InjectEvent(e)
		h.RunFrame()
	}
}

// DriveScroll routes a scroll event at (x,y) with the given delta.
func DriveScroll(h *Harness, x, y, deltaX, deltaY float32) {
	if h == nil {
		return
	}
	Warmup(h)
	h.InjectEvent(Scroll(x, y, deltaX, deltaY))
	h.RunFrame()
}
