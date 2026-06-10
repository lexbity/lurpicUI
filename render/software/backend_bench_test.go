package software

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/fontdata"
	"codeburg.org/lexbit/lurpicui/internal/perfscene"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/text"
)

func BenchmarkSoftware_NodeScene(b *testing.B) {
	for _, nodes := range []int{1000, 10000, 100000} {
		b.Run(perfscene.Describe(nodes), func(b *testing.B) {
			r, _ := benchmarkRenderer(b, 2048, 2048)
			base := perfscene.NodeFrame(nodes, 2048, 2048, 0)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				frame := perfscene.CloneWithNonce(base, uint64(i+1))
				if err := r.Submit(frame); err != nil {
					b.Fatalf("submit: %v", err)
				}
			}
		})
	}
}

func BenchmarkSoftware_ImageScene(b *testing.B) {
	for _, images := range []int{1000, 10000, 100000} {
		b.Run(perfscene.Describe(images), func(b *testing.B) {
			r, _ := benchmarkRenderer(b, 2048, 2048)
			base := perfscene.ImageFrame(images, 2048, 2048, 0)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				frame := perfscene.CloneWithNonce(base, uint64(i+1))
				if err := r.Submit(frame); err != nil {
					b.Fatalf("submit: %v", err)
				}
			}
		})
	}
}

func BenchmarkSoftware_TextScene(b *testing.B) {
	for _, runs := range []int{1000, 10000, 100000} {
		b.Run(perfscene.Describe(runs), func(b *testing.B) {
			r, _ := benchmarkRenderer(b, 2048, 2048)
			base := benchmarkTextFrame(b, runs)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				frame := perfscene.CloneWithNonce(base, uint64(i+1))
				if err := r.Submit(frame); err != nil {
					b.Fatalf("submit: %v", err)
				}
			}
		})
	}
}

func benchmarkRenderer(b *testing.B, w, h int) (*SoftwareRenderer, *testSurface) {
	b.Helper()
	surf := newTestSurface(w, h)
	r := NewSoftwareRenderer()
	if err := r.Initialize(surf); err != nil {
		b.Fatalf("initialize: %v", err)
	}
	return r, surf
}

func benchmarkTextFrame(b *testing.B, runs int) *render.Frame {
	b.Helper()
	reg, err := text.NewFontRegistry()
	if err != nil {
		b.Fatalf("NewFontRegistry: %v", err)
	}
	fontData := fontdata.TestFontBytes()
	if err := reg.LoadFontBytes(fontData, "Noto Sans"); err != nil {
		b.Fatalf("LoadFontBytes: %v", err)
	}
	shaper := text.NewShaper(reg)
	style := text.DefaultStyle()
	style.Family = "Noto Sans"
	style.Size = 14
	layout := shaper.ShapeSimple("The quick brown fox jumps over the lazy dog 0123456789", style)
	if layout == nil || len(layout.Lines) == 0 || len(layout.Lines[0].Runs) == 0 {
		b.Fatal("expected shaped text layout")
	}
	cmds := make([]gfx.Command, 0, runs)
	baseRun := layout.Lines[0].Runs[0]
	for i := 0; i < runs; i++ {
		cmds = append(cmds, gfx.DrawGlyphRun{
			Run:    baseRun,
			Origin: gfx.Point{X: float32((i % 8) * 240), Y: float32((i / 8) * 40)},
			Brush:  gfx.SolidBrush(gfx.Color{R: 1, G: 1, B: 1, A: 1}),
		})
	}
	return &render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:          1,
				Bounds:      gfx.RectFromXYWH(0, 0, 2048, 2048),
				Opacity:     1,
				CommandHash: uint64(runs) << 32,
				Commands:    gfx.CommandList{Commands: cmds},
			},
		},
	}
}
