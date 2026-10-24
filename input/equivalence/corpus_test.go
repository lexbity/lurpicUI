package equivalence

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/drive/proto"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/platform"
)

// corpusTest is the G6.2 corpus: one Pair per mutating verb × representative
// fixture. The drive path goes through the harness's drive sink (the exact
// protocol surface a socket driver uses); the GUI path goes through testkit's
// gestures — the two paths share no input code (QP-14).
func corpusTest() []Pair {
	return []Pair{
		{
			Name:      "input.click/button",
			Fixture:   newButtonFixture,
			GUIHelper: "testkit.DriveClickAt",
			Verb: func(h *testkit.DriveHarness) *proto.Error {
				_, perr := h.Call("input.click", map[string]any{"x": 240, "y": 120})
				return perr
			},
			GUI: func(h *testkit.Harness) { testkit.DriveClickAt(h, 240, 120) },
		},
		{
			Name:      "input.press/slider",
			Fixture:   newSliderFixture,
			GUIHelper: "testkit pointer press",
			Verb: func(h *testkit.DriveHarness) *proto.Error {
				_, perr := h.Call("input.press", map[string]any{"x": 240, "y": 120})
				return perr
			},
			GUI: func(h *testkit.Harness) {
				testkit.Warmup(h)
				h.InjectEvent(testkit.PointerPress(240, 120, 1))
				h.RunFrame()
			},
		},
		{
			Name:      "input.release/slider",
			Fixture:   newSliderFixture,
			GUIHelper: "testkit pointer release",
			Verb: func(h *testkit.DriveHarness) *proto.Error {
				_, perr := h.Call("input.release", map[string]any{"x": 240, "y": 120})
				return perr
			},
			GUI: func(h *testkit.Harness) {
				testkit.Warmup(h)
				h.InjectEvent(testkit.PointerRelease(240, 120, 1))
				h.RunFrame()
			},
		},
		{
			Name:      "input.drag/slider",
			Fixture:   newSliderFixture,
			GUIHelper: "testkit.DriveDragFromTo",
			Verb: func(h *testkit.DriveHarness) *proto.Error {
				_, perr := h.Call("input.drag", map[string]any{
					"from_x": 100, "from_y": 120, "to_x": 300, "to_y": 120,
				})
				return perr
			},
			GUI: func(h *testkit.Harness) { testkit.DriveDragFromTo(h, 100, 120, 300, 120) },
		},
		{
			Name:      "input.move/button",
			Fixture:   newButtonFixture,
			GUIHelper: "testkit.PointerMove + frame",
			Verb: func(h *testkit.DriveHarness) *proto.Error {
				_, perr := h.Call("input.move", map[string]any{"x": 240, "y": 120})
				return perr
			},
			GUI: func(h *testkit.Harness) {
				testkit.Warmup(h)
				h.InjectEvent(testkit.PointerMove(240, 120))
				h.RunFrame()
			},
		},
		{
			Name:      "input.type/textfield",
			Fixture:   newTextFixture,
			GUIHelper: "testkit.DriveType",
			Verb: func(h *testkit.DriveHarness) *proto.Error {
				if _, perr := h.Call("input.focus", map[string]any{
					"target": map[string]string{"testid": "field-b"},
				}); perr != nil {
					return perr
				}
				_, perr := h.Call("input.type", map[string]any{"text": "hello"})
				return perr
			},
			GUI: func(h *testkit.Harness) {
				// Keyboard focus (tab moves A→B) — the route whose focus
				// visuals match input.focus's SetFocus (a click-focus hides
				// the ring by design: focusFromPointer).
				testkit.DriveKeyPress(h, platform.KeyTab, 0)
				testkit.DriveType(h, "hello")
			},
		},
		{
			Name:      "input.focus/textfield",
			Fixture:   newTextFixture,
			GUIHelper: "testkit.DriveClickAt (click focuses)",
			Verb: func(h *testkit.DriveHarness) *proto.Error {
				_, perr := h.Call("input.focus", map[string]any{
					"target": map[string]string{"testid": "field-b"},
				})
				return perr
			},
			GUI: func(h *testkit.Harness) {
				// Keyboard focus via tab (see input.type's twin note).
				testkit.DriveKeyPress(h, platform.KeyTab, 0)
			},
		},
		{
			Name:      "input.scroll/table",
			Fixture:   newScrollFixture,
			GUIHelper: "testkit scroll",
			Verb: func(h *testkit.DriveHarness) *proto.Error {
				_, perr := h.Call("input.scroll", map[string]any{
					"x": 240, "y": 120, "delta_y": 40,
				})
				return perr
			},
			GUI: func(h *testkit.Harness) { testkit.DriveScroll(h, 240, 120, 0, 40) },
		},
		{
			Name:      "cmd.execute/palette",
			Fixture:   newPaletteFixture,
			GUIHelper: "testkit.DriveClickAt (activation path)",
			Verb: func(h *testkit.DriveHarness) *proto.Error {
				_, perr := h.Call("cmd.execute", map[string]any{"id": "toggle.enabled"})
				return perr
			},
			GUI: func(h *testkit.Harness) {
				// The GUI twin exercises the palette's activation path:
				// palette activation dispatches registry.Execute — the same
				// closure the fixture binds to its activation button. The
				// input differs (gesture vs drive verb); the mutation
				// dispatch (registry.Execute) is the app's own surface.
				testkit.DriveClickAt(h, 240, 120)
			},
		},
		{
			Name:      "store.write/value",
			Fixture:   newValueFixture,
			GUIHelper: "testkit.DriveClickAt (checkbox click)",
			Verb: func(h *testkit.DriveHarness) *proto.Error {
				_, perr := h.Call("store.write", map[string]any{
					"name":  "checkbox",
					"value": 1,
				})
				return perr
			},
			GUI:     func(h *testkit.Harness) { testkit.DriveClickAt(h, 240, 120) },
			Compare: CompareStores,
		},
	}
}

// TestCorpus runs every pair (G6.2).
func TestCorpus(t *testing.T) {
	for _, p := range corpusTest() {
		t.Run(p.Name, func(t *testing.T) {
			AssertEquivalent(t, p)
		})
	}
}
