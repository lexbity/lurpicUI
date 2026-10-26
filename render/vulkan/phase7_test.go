//go:build linux && cgo

package vulkan

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/fontdata"
	"codeburg.org/lexbit/lurpicui/internal/testframe"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/text"
)

func TestHeadlessGlyphPacketUpdatesAtlasAndStats(t *testing.T) {
	if err := Shutdown(); err != nil {
		t.Skipf("Vulkan unavailable: %v", err)
	}
	defer func() {
		if err := Shutdown(); err != nil {
			t.Fatalf("final shutdown: %v", err)
		}
	}()
	if err := testResetRustState(); err != nil {
		t.Fatalf("reset rust state: %v", err)
	}

	reg := fontdata.TestFontRegistry(t)
	face := reg.Resolve(text.TextStyle{Family: "Noto Sans", Size: 18})
	run := text.GlyphRun{
		Face:  face,
		Size:  18,
		Style: text.TextStyle{Family: "Noto Sans", Size: 18},
		Glyphs: []text.PositionedGlyph{
			{GlyphID: 65, X: 2.5, Y: 4.25, Advance: 8.5},
		},
	}

	backend := &Backend{}
	if err := backend.Initialize(nil); err != nil {
		t.Skipf("Vulkan unavailable: %v", err)
	}
	defer backend.Destroy()

	frame := testframe.Seal(render.FrameInput{
		Batches: []render.BatchInput{
			{
				ID:      1,
				Bounds:  gfx.RectFromXYWH(0, 0, 64, 64),
				Opacity: 1,

				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.DrawGlyphRun{
						Run:    run,
						Origin: gfx.Point{X: 10, Y: 12},
						Brush:  gfx.SolidBrush(gfx.Color{R: 1, A: 1}),
					},
				}},
			},
		},
	})

	defer frame.Release()
	packet, err := backend.encoder.Encode(frame, backend.images)
	if err != nil {
		t.Fatal(err)
	}
	if err := SubmitFrame(packet); err != nil {
		t.Fatalf("headless packet admission: %v", err)
	}
	if got, err := testLastBatchCount(); err != nil {
		t.Fatalf("last batch count: %v", err)
	} else if got != 1 {
		t.Fatalf("last batch count = %d, want 1", got)
	}
	if got, err := testLastCommandCount(); err != nil {
		t.Fatalf("last command count: %v", err)
	} else if got != 1 {
		t.Fatalf("last command count = %d, want 1", got)
	}
	if got, err := testGlyphAtlasCount(); err != nil {
		t.Fatalf("glyph atlas count: %v", err)
	} else if got != 1 {
		t.Fatalf("glyph atlas count = %d, want 1", got)
	}
}
