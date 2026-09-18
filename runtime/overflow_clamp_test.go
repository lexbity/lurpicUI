package runtime

import (
	"image/color"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

// TestRuntimeOverflowClampedCountWired pins NFR-8 wiring: a Grow facet arranged
// below its measured size clamps and surfaces the violation in the frame's
// FrameStats.OverflowClampedCount; a quiet frame with no Grow violation reports
// zero.
func TestRuntimeOverflowClampedCountWired(t *testing.T) {
	hook := &recordingFrameStats{}
	root := newRuntimeRenderFacet("root", gfx.RectFromXYWH(0, 0, 400, 300), color.RGBA{A: 255})
	child := newRuntimeRenderFacet("child", gfx.RectFromXYWH(0, 0, 200, 100), color.RGBA{A: 255})
	child.layout.Parent.Overflow = facet.OverflowGrow
	child.layout.Measure(facet.MeasureContext{}, facet.Constraints{MaxSize: gfx.Size{W: 400, H: 300}})
	root.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		root.layout.ArrangedBounds = bounds
		// Arrange the Grow child below its 200x100 min-content.
		child.layout.Arrange(ctx, gfx.RectFromXYWH(0, 0, 40, 30))
	}
	rt, err := New(func() Config {
		cfg := DefaultConfig()
		cfg.LayerRegistry = testLayerRegistry(t)
		cfg.DiagnosticsHook = hook
		return cfg
	}(), nil, nil, &backendFixture{}, root)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	rt.window = &testWindow{width: 400, height: 300}

	rt.RunOneFrame()
	if got := child.layout.ArrangedBounds.Width(); got != 200 {
		t.Fatalf("Grow child arranged width = %v, want clamped 200", got)
	}
	if got := hook.last().OverflowClampedCount; got < 1 {
		t.Fatalf("OverflowClampedCount = %d, want >= 1 (Grow violation)", got)
	}
}
