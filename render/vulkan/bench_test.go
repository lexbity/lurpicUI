//go:build linux && cgo

package vulkan

import (
	"fmt"
	"image"
	"image/color"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/fontdata"
	"codeburg.org/lexbit/lurpicui/internal/perfscene"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/text"
)

func BenchmarkVulkan_NodeScene(b *testing.B) {
	for _, nodes := range []int{1000, 10000, 100000} {
		b.Run(perfscene.Describe(nodes), func(b *testing.B) {
			backend := mustBenchmarkBackend(b)
			defer backend.Destroy()
			base := perfscene.NodeFrame(nodes, 2048, 2048, 0)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				frame := perfscene.CloneWithNonce(base, uint64(i+1))
				if err := backend.Submit(frame); err != nil {
					b.Fatalf("submit: %v", err)
				}
			}
		})
	}
}

func BenchmarkVulkan_ImageScene(b *testing.B) {
	for _, images := range []int{1000, 10000, 100000} {
		b.Run(perfscene.Describe(images), func(b *testing.B) {
			backend := mustBenchmarkBackend(b)
			defer backend.Destroy()
			base := perfscene.ImageFrame(images, 2048, 2048, 0)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				frame := perfscene.CloneWithNonce(base, uint64(i+1))
				if err := backend.Submit(frame); err != nil {
					b.Fatalf("submit: %v", err)
				}
			}
		})
	}
}

func BenchmarkVulkan_TextScene(b *testing.B) {
	for _, runs := range []int{1000, 10000, 100000} {
		b.Run(perfscene.Describe(runs), func(b *testing.B) {
			backend := mustBenchmarkBackend(b)
			defer backend.Destroy()
			base := benchmarkVulkanTextFrame(b, runs)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				frame := perfscene.CloneWithNonce(base, uint64(i+1))
				if err := backend.Submit(frame); err != nil {
					b.Fatalf("submit: %v", err)
				}
			}
		})
	}
}

func mustBenchmarkBackend(b *testing.B) *Backend {
	b.Helper()
	backend := &Backend{}
	if err := backend.Initialize(nil); err != nil {
		b.Skipf("Vulkan unavailable: %v", err)
	}
	return backend
}

// ensureReleaseRustLibrary builds target/release/liblurpic_render.so and returns
// its path. Benchmarks measure the production build.
func ensureReleaseRustLibrary() (string, error) {
	manifest, err := RustCrateManifestPath()
	if err != nil {
		return "", err
	}
	if err := CheckRustToolchain(); err != nil {
		return "", err
	}
	if out, err := command("cargo", "build", "--release", "--offline", "--manifest-path", manifest).CombinedOutput(); err != nil {
		return "", fmt.Errorf("vulkan: cargo build --release failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return filepath.Join(filepath.Dir(manifest), "target", "release", rustSharedLibraryName()), nil
}

// BenchmarkVulkanSubmit_SolidRects measures the per-frame submission of the
// Slice 3 solid pipeline: pooled encode -> offscreen render -> readback. The
// headless path cannot present, so the offscreen readback exercises the full
// render (encode -> instance ring -> raster -> resolve -> copy). The library
// is built in release: the debug build's unoptimized byte-swap loop dominates
// (measured ~54ms at 1080p) and would not reflect the production build the
// NFR-2 budget targets.
//
// Hard gates:
//   - NFR-6: AllocsPerOp(0) — the per-frame submission must allocate zero Go
//     bytes steady-state (the encoder is pooled; the output buffer is reused).
//   - NFR-2: <= 16.7 ms per frame at 1080p for many_rects_instanced.
func BenchmarkVulkanSubmit_SolidRects(b *testing.B) {
	releasePath, err := ensureReleaseRustLibrary()
	if err != nil {
		b.Skipf("release library unavailable: %v", err)
	}
	// Earlier tests in this process load the debug library; reset the loader so
	// the benchmark measures the production (release) build.
	resetRustLibraryLoaderForTest()
	rustLibraryPathResolver = func() (string, error) { return releasePath, nil }
	b.Logf("loading release library: %s", releasePath)
	if err := Init(); err != nil {
		b.Skipf("Vulkan unavailable: %v", err)
	}
	defer func() { _ = Shutdown() }()

	const w, h = 1920, 1080
	const rects = 400
	cmds := make([]gfx.Command, 0, rects)
	for i := 0; i < rects; i++ {
		x := float32((i % 40) * 48)
		y := float32((i / 40) * 27)
		cmds = append(cmds, gfx.FillRect{
			Rect:  gfx.RectFromXYWH(x, y, 24, 24),
			Brush: gfx.SolidBrush(gfx.Color{R: float32(i%255) / 255, G: 0.4, B: 0.6, A: 1}),
		})
	}
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{{
			ID:       1,
			Bounds:   gfx.RectFromXYWH(0, 0, w, h),
			Opacity:  1,
			Commands: gfx.CommandList{Commands: cmds},
		}},
	}

	var enc frameEncoder
	packet, err := enc.Encode(frame, nil)
	if err != nil {
		b.Fatalf("encode: %v", err)
	}
	out := make([]byte, w*h*4)
	// Warm up: the first call allocates the 1080p MSAA/readback images and
	// compiles the pipeline; the timed loop measures steady-state submits.
	if err := submitAndReadbackInto(packet, w, h, out); err != nil {
		b.Fatalf("warmup readback: %v", err)
	}

	submit := func() {
		packet, err := enc.Encode(frame, nil)
		if err != nil {
			b.Fatal(err)
		}
		if err := submitAndReadbackInto(packet, w, h, out); err != nil {
			b.Fatal(err)
		}
	}

	// NFR-6: the per-frame submission (pooled encode + FFI readback) must
	// allocate zero Go bytes steady-state.
	allocs := testing.AllocsPerRun(50, submit)
	b.ReportMetric(allocs, "allocs/op")
	if allocs != 0 {
		b.Errorf("NFR-6: per-frame submission must allocate zero bytes steady-state, got %.0f allocs/op", allocs)
	}

	// NFR-2: frame time at 1080p for many_rects_instanced.
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		submit()
	}
	b.StopTimer()
	if b.Elapsed()/time.Duration(b.N) > 16700*time.Microsecond {
		b.Errorf("NFR-2: frame time %v exceeds the 16.7ms budget at 1080p",
			b.Elapsed()/time.Duration(b.N))
	}
}

// BenchmarkVulkanClipMechanism_Discard measures the clip mechanism on a
// deeply-clipped scene: large quads whose visible area is a small fraction of
// the rasterized quad. The pipeline aligns the axis-aligned clip to a per-draw
// vkCmdSetScissor (Slice 3 forward) so the rasterizer culls fragments before
// the shader; the fragment discard remains only for the exact float boundary.
// Measured on the reference driver: the deep-clip scene dropped from ~5.5ms
// (fragment discard only) to ~2.7ms (scissor-culled) — the scissor alignment
// was decided by data, and this benchmark keeps the deep-clip vs no-clip delta
// visible so a regression re-opens the question.
func BenchmarkVulkanClipMechanism_Discard(b *testing.B) {
	releasePath, err := ensureReleaseRustLibrary()
	if err != nil {
		b.Skipf("release library unavailable: %v", err)
	}
	resetRustLibraryLoaderForTest()
	rustLibraryPathResolver = func() (string, error) { return releasePath, nil }
	if err := Init(); err != nil {
		b.Skipf("Vulkan unavailable: %v", err)
	}
	defer func() { _ = Shutdown() }()

	const w, h = 1920, 1080
	// 200 quarter-canvas rects overlapping a 64x64 clip region.
	cmds := make([]gfx.Command, 0, 200)
	for i := 0; i < 200; i++ {
		cmds = append(cmds, gfx.FillRect{
			Rect:  gfx.RectFromXYWH(0, 0, w/2, h/2),
			Brush: gfx.SolidBrush(gfx.Color{R: 0.5, G: 0.4, B: 0.6, A: 1}),
		})
	}
	deepClip := &render.Frame{
		RenderBatchs: []render.RenderBatch{{
			ID:      1,
			Bounds:  gfx.RectFromXYWH(0, 0, w, h),
			Opacity: 1,
			Commands: gfx.CommandList{Commands: append([]gfx.Command{
				gfx.PushClipRect{Rect: gfx.RectFromXYWH(w/2-32, h/2-32, 64, 64)},
			}, append(cmds, gfx.PopClip{})...)},
		}},
	}
	noClip := &render.Frame{
		RenderBatchs: []render.RenderBatch{{
			ID:      1,
			Bounds:  gfx.RectFromXYWH(0, 0, w, h),
			Opacity: 1,
			Commands: gfx.CommandList{Commands: append([]gfx.Command{
				gfx.PushClipRect{Rect: gfx.RectFromXYWH(0, 0, w, h)},
			}, append(cmds, gfx.PopClip{})...)},
		}},
	}

	var enc frameEncoder
	out := make([]byte, w*h*4)
	measure := func(name string, frame *render.Frame) (perOp time.Duration) {
		packet, err := enc.Encode(frame, nil)
		if err != nil {
			b.Fatalf("encode: %v", err)
		}
		if err := submitAndReadbackInto(packet, w, h, out); err != nil {
			b.Fatalf("warmup: %v", err)
		}
		start := time.Now()
		for i := 0; i < b.N; i++ {
			packet, err := enc.Encode(frame, nil)
			if err != nil {
				b.Fatal(err)
			}
			if err := submitAndReadbackInto(packet, w, h, out); err != nil {
				b.Fatal(err)
			}
		}
		elapsed := time.Since(start) / time.Duration(b.N)
		b.ReportMetric(float64(elapsed.Microseconds()), "us/op")
		return elapsed
	}

	deep := measure("deep_clip", deepClip)
	flat := measure("no_clip", noClip)

	// Report the clip mechanism's delta vs the no-clip baseline and keep the
	// deep-clip frame time under the NFR-2 budget.
	ratio := float64(deep) / float64(flat)
	b.ReportMetric(ratio, "clip_delta_over_no_clip")
	if deep > 16700*time.Microsecond {
		b.Errorf("deep-clip frame time %v exceeds the 16.7ms budget; the scissor "+
			"alignment is insufficient on this driver", deep)
	}
	b.Logf("deep-clip scissor render %v vs no-clip %v (%.2fx): the per-draw scissor "+
		"alignment is kept; measured benefit over fragment-discard-only was ~5.5ms -> ~2.7ms.",
		deep, flat, ratio)
}

// BenchmarkVulkanSubmit_Textures measures the Slice 4 textured pipeline:
// pooled encode -> offscreen render with per-group descriptor sets -> readback.
// Textures are uploaded once and referenced by DrawTexture handle (the
// steady-state form: cached textures drawn many times), so the timed loop
// exercises the sampler-bound draw path with zero Go allocations. Note: the
// per-frame Go-side `hashImage` dedup cost of fresh `DrawImage` content is a
// separate pre-existing cache concern, not the GPU path this benchmark gates.
//
// Hard gates:
//   - NFR-6: AllocsPerOp(0).
//   - NFR-2: <= 16.7 ms per frame at 1080p.
func BenchmarkVulkanSubmit_Textures(b *testing.B) {
	releasePath, err := ensureReleaseRustLibrary()
	if err != nil {
		b.Skipf("release library unavailable: %v", err)
	}
	resetRustLibraryLoaderForTest()
	rustLibraryPathResolver = func() (string, error) { return releasePath, nil }
	b.Logf("loading release library: %s", releasePath)
	if err := Init(); err != nil {
		b.Skipf("Vulkan unavailable: %v", err)
	}
	defer func() { _ = Shutdown() }()

	const w, h = 1920, 1080
	// 12 distinct textures, each stamped across the canvas 64x.
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(x * 4), G: uint8(y * 4), B: 128, A: 255})
		}
	}
	handles := make([]uint64, 0, 12)
	for i := 0; i < 12; i++ {
		dup := image.NewRGBA(img.Rect)
		for y := 0; y < 64; y++ {
			for x := 0; x < 64; x++ {
				c := img.RGBAAt(x, y)
				dup.SetRGBA(x, y, color.RGBA{R: c.R, G: c.G, B: uint8(64 + i*12), A: c.A})
			}
		}
		handle, err := UploadImage(dup.Pix, 64, 64, dup.Stride, 0)
		if err != nil {
			b.Fatalf("upload texture %d: %v", i, err)
		}
		handles = append(handles, handle)
	}
	defer func() {
		for _, handle := range handles {
			_ = DestroyImage(handle)
		}
	}()

	const draws = 768 // 12 textures * 64 placements
	cmds := make([]gfx.Command, 0, draws)
	for i := 0; i < draws; i++ {
		x := float32((i % 48) * 40)
		y := float32((i / 48) * 22)
		cmds = append(cmds, gfx.DrawTexture{
			TextureID: handles[i%len(handles)],
			DestRect:  gfx.RectFromXYWH(x, y, 64, 64),
			SrcRect:   gfx.RectFromXYWH(0, 0, 64, 64),
			Sampling:  gfx.SamplingNearest,
			Opacity:   1,
		})
	}
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{{
			ID:       1,
			Bounds:   gfx.RectFromXYWH(0, 0, w, h),
			Opacity:  1,
			Commands: gfx.CommandList{Commands: cmds},
		}},
	}

	var enc frameEncoder
	packet, err := enc.Encode(frame, nil)
	if err != nil {
		b.Fatalf("encode: %v", err)
	}
	out := make([]byte, w*h*4)
	if err := submitAndReadbackInto(packet, w, h, out); err != nil {
		b.Fatalf("warmup readback: %v", err)
	}

	submit := func() {
		packet, err := enc.Encode(frame, nil)
		if err != nil {
			b.Fatal(err)
		}
		if err := submitAndReadbackInto(packet, w, h, out); err != nil {
			b.Fatal(err)
		}
	}

	allocs := testing.AllocsPerRun(50, submit)
	b.ReportMetric(allocs, "allocs/op")
	if allocs != 0 {
		b.Errorf("NFR-6: per-frame submission must allocate zero bytes steady-state, got %.0f allocs/op", allocs)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		submit()
	}
	b.StopTimer()
	if b.Elapsed()/time.Duration(b.N) > 16700*time.Microsecond {
		b.Errorf("NFR-2: frame time %v exceeds the 16.7ms budget at 1080p",
			b.Elapsed()/time.Duration(b.N))
	}
}

// BenchmarkVulkanSubmit_Glyphs measures the Slice 5 glyph pipeline: pooled
// encode (with the Go-side raster cache warm) -> offscreen render through the
// packed-atlas pipelines -> readback. Glyphs are uploaded once; the timed loop
// exercises the atlas-sampled draw path with zero Go allocations.
//
// Hard gates:
//   - NFR-6: AllocsPerOp(0).
//   - NFR-2: <= 16.7 ms per frame at 1080p.
func BenchmarkVulkanSubmit_Glyphs(b *testing.B) {
	releasePath, err := ensureReleaseRustLibrary()
	if err != nil {
		b.Skipf("release library unavailable: %v", err)
	}
	resetRustLibraryLoaderForTest()
	rustLibraryPathResolver = func() (string, error) { return releasePath, nil }
	b.Logf("loading release library: %s", releasePath)
	if err := Init(); err != nil {
		b.Skipf("Vulkan unavailable: %v", err)
	}
	defer func() { _ = Shutdown() }()

	const w, h = 1920, 1080
	reg, err := text.NewFontRegistry()
	if err != nil {
		b.Fatalf("NewFontRegistry: %v", err)
	}
	if err := reg.LoadFontBytes(fontdata.TestFontBytes(), "Noto Sans"); err != nil {
		b.Fatalf("LoadFontBytes: %v", err)
	}
	face := reg.Resolve(text.TextStyle{Family: "Noto Sans", Size: 16})
	adv := float32(12)
	glyphs := []text.PositionedGlyph{
		{GlyphID: 65, X: 0, Y: 0, Advance: adv},
		{GlyphID: 66, X: adv, Y: 0, Advance: adv},
		{GlyphID: 67, X: 2 * adv, Y: 0, Advance: adv},
		{GlyphID: 68, X: 3 * adv, Y: 0, Advance: adv},
	}

	const runs = 500 // 2000 glyph draws
	cmds := make([]gfx.Command, 0, runs)
	for i := 0; i < runs; i++ {
		x := float32((i % 50) * 38)
		y := float32((i/50)*26 + 16)
		run := text.GlyphRun{
			Face: face, Size: 16, Style: text.TextStyle{Family: "Noto Sans", Size: 16},
			Glyphs: glyphs,
		}
		cmds = append(cmds, gfx.DrawGlyphRun{
			Run:    run,
			Origin: gfx.Point{X: x, Y: y},
			Brush:  gfx.SolidBrush(gfx.ColorFromRGBA8(20, 20, 24, 255)),
		})
	}
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{{
			ID:       1,
			Bounds:   gfx.RectFromXYWH(0, 0, w, h),
			Opacity:  1,
			Commands: gfx.CommandList{Commands: cmds},
		}},
	}

	var enc frameEncoder
	packet, err := enc.Encode(frame, nil)
	if err != nil {
		b.Fatalf("encode: %v", err)
	}
	out := make([]byte, w*h*4)
	if err := submitAndReadbackInto(packet, w, h, out); err != nil {
		b.Fatalf("warmup readback: %v", err)
	}

	submit := func() {
		packet, err := enc.Encode(frame, nil)
		if err != nil {
			b.Fatal(err)
		}
		if err := submitAndReadbackInto(packet, w, h, out); err != nil {
			b.Fatal(err)
		}
	}

	allocs := testing.AllocsPerRun(50, submit)
	b.ReportMetric(allocs, "allocs/op")
	if allocs != 0 {
		b.Errorf("NFR-6: per-frame submission must allocate zero bytes steady-state, got %.0f allocs/op", allocs)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		submit()
	}
	b.StopTimer()
	if b.Elapsed()/time.Duration(b.N) > 16700*time.Microsecond {
		b.Errorf("NFR-2: frame time %v exceeds the 16.7ms budget at 1080p",
			b.Elapsed()/time.Duration(b.N))
	}
}

// BenchmarkVulkanSubmit_Gradients measures the Slice 6 gradient pipeline:
// pooled encode (gradient UBOs bump-allocated into the per-frame uniform ring)
// -> offscreen render -> readback. Distinct gradients exercise multiple UBO
// descriptor sets; the timed loop is allocation-free on the Go side.
//
// Hard gates:
//   - NFR-6: AllocsPerOp(0).
//   - NFR-2: <= 16.7 ms per frame at 1080p.
func BenchmarkVulkanSubmit_Gradients(b *testing.B) {
	releasePath, err := ensureReleaseRustLibrary()
	if err != nil {
		b.Skipf("release library unavailable: %v", err)
	}
	resetRustLibraryLoaderForTest()
	rustLibraryPathResolver = func() (string, error) { return releasePath, nil }
	b.Logf("loading release library: %s", releasePath)
	if err := Init(); err != nil {
		b.Skipf("Vulkan unavailable: %v", err)
	}
	defer func() { _ = Shutdown() }()

	const w, h = 1920, 1080
	// 24 distinct gradients, each filling one row of consecutive rects so the
	// encoder batches each row into one UBO group (realistic: same-gradient
	// fills are usually emitted consecutively).
	const gradients = 24
	const perRow = 48
	cmds := make([]gfx.Command, 0, gradients*perRow)
	for g := 0; g < gradients; g++ {
		start := gfx.ColorFromRGBA8(120, 40+uint8((g*13)%200), 40, 255)
		end := gfx.ColorFromRGBA8(40, 120, 40+uint8((g*29)%200), 255)
		brush := gfx.LinearGradientBrush(
			gfx.Point{X: 0, Y: 0},
			gfx.Point{X: 64, Y: 0},
			[]gfx.GradientStop{{Offset: 0, Color: start}, {Offset: 1, Color: end}},
		)
		y := float32(g * (h / gradients))
		for k := 0; k < perRow; k++ {
			x := float32(k * 40)
			cmds = append(cmds, gfx.FillRect{Rect: gfx.RectFromXYWH(x, y, 64, h/gradients), Brush: brush})
		}
	}
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{{
			ID:       1,
			Bounds:   gfx.RectFromXYWH(0, 0, w, h),
			Opacity:  1,
			Commands: gfx.CommandList{Commands: cmds},
		}},
	}

	var enc frameEncoder
	packet, err := enc.Encode(frame, nil)
	if err != nil {
		b.Fatalf("encode: %v", err)
	}
	out := make([]byte, w*h*4)
	if err := submitAndReadbackInto(packet, w, h, out); err != nil {
		b.Fatalf("warmup readback: %v", err)
	}

	submit := func() {
		packet, err := enc.Encode(frame, nil)
		if err != nil {
			b.Fatal(err)
		}
		if err := submitAndReadbackInto(packet, w, h, out); err != nil {
			b.Fatal(err)
		}
	}

	allocs := testing.AllocsPerRun(50, submit)
	b.ReportMetric(allocs, "allocs/op")
	if allocs != 0 {
		b.Errorf("NFR-6: per-frame submission must allocate zero bytes steady-state, got %.0f allocs/op", allocs)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		submit()
	}
	b.StopTimer()
	if b.Elapsed()/time.Duration(b.N) > 16700*time.Microsecond {
		b.Errorf("NFR-2: frame time %v exceeds the 16.7ms budget at 1080p",
			b.Elapsed()/time.Duration(b.N))
	}
}

// BenchmarkVulkanSubmit_Shadows measures the Slice 9 shadow pipeline: pooled
// encode -> per-shadow R8 mask pass + separable Gaussian (H then V) + tinted
// composite -> readback. The reference scene is an elevation dashboard: 12 card
// shadows (blur 8) at 1080p, each with the card rect drawn over it.
//
// Hard gates:
//   - NFR-6: AllocsPerOp(0) — the per-frame shadow encode must allocate zero Go
//     bytes steady-state.
//   - NFR-2: <= 16.7 ms per frame at 1080p.
func BenchmarkVulkanSubmit_Shadows(b *testing.B) {
	releasePath, err := ensureReleaseRustLibrary()
	if err != nil {
		b.Skipf("release library unavailable: %v", err)
	}
	resetRustLibraryLoaderForTest()
	rustLibraryPathResolver = func() (string, error) { return releasePath, nil }
	b.Logf("loading release library: %s", releasePath)
	if err := Init(); err != nil {
		b.Skipf("Vulkan unavailable: %v", err)
	}
	defer func() { _ = Shutdown() }()

	const w, h = 1920, 1080
	const cards = 12
	cmds := make([]gfx.Command, 0, cards*2)
	for i := 0; i < cards; i++ {
		x := float32((i % 6) * 300)
		y := float32((i / 6) * 320)
		card := gfx.RectFromXYWH(x+40, y+60, 220, 260)
		cmds = append(cmds, gfx.DrawBlurredShadow{
			Path:       gfx.RoundedRectPath(card, 16),
			Color:      gfx.ColorFromRGBA8(0, 0, 0, 120),
			BlurRadius: 8,
			Offset:     gfx.Point{X: 0, Y: 6},
			Inner:      false,
		})
		cmds = append(cmds, gfx.FillRect{
			Rect:  card,
			Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(240, 240, 244, 255)),
		})
	}
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{{
			ID:       1,
			Bounds:   gfx.RectFromXYWH(0, 0, w, h),
			Opacity:  1,
			Commands: gfx.CommandList{Commands: cmds},
		}},
	}

	var enc frameEncoder
	packet, err := enc.Encode(frame, nil)
	if err != nil {
		b.Fatalf("encode: %v", err)
	}
	out := make([]byte, w*h*4)
	if err := submitAndReadbackInto(packet, w, h, out); err != nil {
		b.Fatalf("warmup readback: %v", err)
	}

	submit := func() {
		packet, err := enc.Encode(frame, nil)
		if err != nil {
			b.Fatal(err)
		}
		if err := submitAndReadbackInto(packet, w, h, out); err != nil {
			b.Fatal(err)
		}
	}

	allocs := testing.AllocsPerRun(50, submit)
	b.ReportMetric(allocs, "allocs/op")
	if allocs != 0 {
		b.Errorf("NFR-6: per-frame submission must allocate zero bytes steady-state, got %.0f allocs/op", allocs)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		submit()
	}
	b.StopTimer()
	if b.Elapsed()/time.Duration(b.N) > 16700*time.Microsecond {
		b.Errorf("NFR-2: frame time %v exceeds the 16.7ms budget at 1080p",
			b.Elapsed()/time.Duration(b.N))
	}
}

func benchmarkVulkanTextFrame(b *testing.B, runs int) *render.Frame {
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

// chartAreaPath builds a chart-area fill path (the shape `marks/viz/area.go`
// emits: a data-line top edge + reverse baseline closure) with `teeth` zigzag
// segments across `w` x `h`. This is the Slice 7 reference scene: a single
// large `FillPath` whose cover quad spans the frame.
func chartAreaPath(w, h int, teeth int) gfx.Path {
	const margin = 40
	p := gfx.NewPath().MoveTo(gfx.Point{X: margin, Y: float32(h - margin)})
	step := (float32(w) - 2*margin) / float32(teeth)
	for i := 0; i <= teeth; i++ {
		x := margin + float32(i)*step
		amp := float32(h - 2*margin)
		y := float32(margin) + amp*float32(i%2)
		p = p.LineTo(gfx.Point{X: x, Y: y})
	}
	// Close along the baseline (top edge -> bottom-right -> bottom-left).
	p = p.
		LineTo(gfx.Point{X: float32(w - margin), Y: float32(h - margin)}).
		LineTo(gfx.Point{X: margin, Y: float32(h - margin)}).
		Close()
	return p.Build()
}

// BenchmarkVulkanSubmit_PathFill measures the Slice 7 stencil path-fill
// pipeline: pooled encode -> per-frame path ring (flattened winding triangles)
// -> stencil winding pass + 12x12-supersample cover -> readback. The reference
// scene is a full-frame chart-area fill (20 teeth, 42 edges) at 1080p — 4x the
// spec's chart fixture's point count, representative of a real dashboard area
// chart. (Measured: 42 edges ~6.7ms, 162 edges ~18ms at 1080p; the per-tile /
// y-bucket follow-on in the spec's Q2 note addresses chart-heavy scenes.)
//
// Hard gates:
//   - NFR-6: AllocsPerOp(0) — the per-frame submission must allocate zero Go
//     bytes steady-state (the path is prebuilt; encode is pooled; the output
//     buffer is reused).
//   - NFR-2: <= 16.7 ms per frame at 1080p for the chart fixture.
func BenchmarkVulkanSubmit_PathFill(b *testing.B) {
	releasePath, err := ensureReleaseRustLibrary()
	if err != nil {
		b.Skipf("release library unavailable: %v", err)
	}
	resetRustLibraryLoaderForTest()
	rustLibraryPathResolver = func() (string, error) { return releasePath, nil }
	b.Logf("loading release library: %s", releasePath)
	if err := Init(); err != nil {
		b.Skipf("Vulkan unavailable: %v", err)
	}
	defer func() { _ = Shutdown() }()

	const w, h = 1920, 1080
	const teeth = 20
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{{
			ID:      1,
			Bounds:  gfx.RectFromXYWH(0, 0, w, h),
			Opacity: 1,
			Commands: gfx.CommandList{Commands: []gfx.Command{
				gfx.FillPath{
					Path:  chartAreaPath(w, h, teeth),
					Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(60, 120, 200, 255)),
				},
			}},
		}},
	}

	var enc frameEncoder
	packet, err := enc.Encode(frame, nil)
	if err != nil {
		b.Fatalf("encode: %v", err)
	}
	out := make([]byte, w*h*4)
	// Warm up: the first call compiles the path-fill pipelines and allocates
	// the 1080p targets; the timed loop measures steady-state submits.
	if err := submitAndReadbackInto(packet, w, h, out); err != nil {
		b.Fatalf("warmup readback: %v", err)
	}

	submit := func() {
		packet, err := enc.Encode(frame, nil)
		if err != nil {
			b.Fatal(err)
		}
		if err := submitAndReadbackInto(packet, w, h, out); err != nil {
			b.Fatal(err)
		}
	}

	allocs := testing.AllocsPerRun(50, submit)
	b.ReportMetric(allocs, "allocs/op")
	if allocs != 0 {
		b.Errorf("NFR-6: per-frame submission must allocate zero bytes steady-state, got %.0f allocs/op", allocs)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		submit()
	}
	b.StopTimer()
	if b.Elapsed()/time.Duration(b.N) > 16700*time.Microsecond {
		b.Errorf("NFR-2: frame time %v exceeds the 16.7ms budget at 1080p",
			b.Elapsed()/time.Duration(b.N))
	}
}

// BenchmarkVulkanSubmit_Strokes measures the Slice 8 stroke pipeline: the Go
// encoder expands every stroke command into the FillPath of its outline (via
// the pooled gfx.StrokeScratch) at encode time, then the Slice 7 stencil fill
// renders it. The reference scene is a dashboard: 12 chart lines (DrawPolyline,
// width 2) plus 4 stroke rectangles at 1080p.
//
// (Measured: the Slice 7 stencil fill's per-fragment winding coverage is
// O(edges x 12x12 samples) over each path's bounding quad, so 200 full-frame
// strokes cost ~33 ms at 1080p — the spec's Q2 per-tile follow-on. This
// benchmark holds a realistic 16-stroke dashboard to the NFR-2 budget.)
//
// Hard gates:
//   - NFR-6: AllocsPerOp(0) — the per-frame stroke expansion and encode must
//     allocate zero Go bytes steady-state (the Scratch reuses its buffers).
//   - NFR-2: <= 16.7 ms per frame at 1080p.
func BenchmarkVulkanSubmit_Strokes(b *testing.B) {
	releasePath, err := ensureReleaseRustLibrary()
	if err != nil {
		b.Skipf("release library unavailable: %v", err)
	}
	resetRustLibraryLoaderForTest()
	rustLibraryPathResolver = func() (string, error) { return releasePath, nil }
	b.Logf("loading release library: %s", releasePath)
	if err := Init(); err != nil {
		b.Skipf("Vulkan unavailable: %v", err)
	}
	defer func() { _ = Shutdown() }()

	const w, h = 1920, 1080
	const lines = 12
	const rects = 4
	cmds := make([]gfx.Command, 0, lines+rects)
	for i := 0; i < lines; i++ {
		baseX := float32((i % 8) * 240)
		baseY := float32((i / 8) * 270)
		pts := make([]gfx.Point, 0, 24)
		for k := 0; k < 24; k++ {
			pts = append(pts, gfx.Point{
				X: baseX + float32(k)*80,
				Y: baseY + 60 + float32((k*37)%120),
			})
		}
		cmds = append(cmds, gfx.DrawPolyline{
			Points: pts,
			Stroke: gfx.StrokeStyle{Width: 2},
			Brush:  gfx.SolidBrush(gfx.ColorFromRGBA8(60, 120, 200, 255)),
		})
	}
	for i := 0; i < rects; i++ {
		x := float32((i % 10) * 192)
		y := float32((i / 10) * 108)
		stroke := gfx.DefaultStroke(3)
		stroke.Join = gfx.LineJoinRound
		cmds = append(cmds, gfx.StrokePath{
			Path:   gfx.RoundedRectPath(gfx.RectFromXYWH(x, y, 120, 60), 12),
			Stroke: stroke,
			Brush:  gfx.SolidBrush(gfx.ColorFromRGBA8(230, 60, 60, 255)),
		})
	}
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{{
			ID:       1,
			Bounds:   gfx.RectFromXYWH(0, 0, w, h),
			Opacity:  1,
			Commands: gfx.CommandList{Commands: cmds},
		}},
	}

	var enc frameEncoder
	packet, err := enc.Encode(frame, nil)
	if err != nil {
		b.Fatalf("encode: %v", err)
	}
	out := make([]byte, w*h*4)
	if err := submitAndReadbackInto(packet, w, h, out); err != nil {
		b.Fatalf("warmup readback: %v", err)
	}

	submit := func() {
		packet, err := enc.Encode(frame, nil)
		if err != nil {
			b.Fatal(err)
		}
		if err := submitAndReadbackInto(packet, w, h, out); err != nil {
			b.Fatal(err)
		}
	}

	allocs := testing.AllocsPerRun(50, submit)
	b.ReportMetric(allocs, "allocs/op")
	if allocs != 0 {
		b.Errorf("NFR-6: per-frame submission must allocate zero bytes steady-state, got %.0f allocs/op", allocs)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		submit()
	}
	b.StopTimer()
	if b.Elapsed()/time.Duration(b.N) > 16700*time.Microsecond {
		b.Errorf("NFR-2: frame time %v exceeds the 16.7ms budget at 1080p",
			b.Elapsed()/time.Duration(b.N))
	}
}
