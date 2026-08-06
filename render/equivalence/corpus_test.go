package equivalence_test

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/fontdata"
	"codeburg.org/lexbit/lurpicui/render/equivalence"
	"codeburg.org/lexbit/lurpicui/render/equivalence/corpus"
	"codeburg.org/lexbit/lurpicui/render/vulkan"
)

// requireVulkanRaster builds the Rust library if needed and skips the test when
// no usable renderer is available (the CPU readback path does not need a GPU).
func requireVulkanRaster(t *testing.T) {
	t.Helper()
	if _, err := vulkan.Version(); err != nil {
		if buildErr := vulkan.BuildRustLibrary(); buildErr != nil {
			t.Skipf("Rust raster renderer unavailable: %v (build: %v)", err, buildErr)
		}
	}
	if _, err := vulkan.Version(); err != nil {
		t.Skipf("Rust raster renderer unavailable after build: %v", err)
	}
}

// deferredFixtures are corpus entries that exercise a command at the wire level
// but are not yet rendered by the current Vulkan pipeline. They are kept so the
// corpus covers every gfx command; the runner skips them until the slice that
// renders them lands.
var deferredFixtures = map[string]string{
	"image_bilinear_upscale":   "bilinear needs the software oracle to honor Sampling (software backend unchanged in Slice 4); GPU path verified by TestDrawImage_Bilinear",
	"image_bilinear_downscale": "bilinear needs the software oracle to honor Sampling (software backend unchanged in Slice 4); GPU path verified by TestDrawImage_Bilinear",
	"texture_nearest_1to1":     "DrawTexture renders via TestDrawTexture_Rendered (backend-specific texture handles)",
}

// featureTolerances relax the Q1 default only for fixtures whose edge pixels
// are governed by a documented coverage-AA model difference (Q1: "record a
// feature-specific tolerance with measured justification — never silently
// widened").
//
//   - solid_rect_rotated_45: the GPU analytic coverage-AA (Q8 amendment) vs the
//     software oracle's exact polygon-area coverage on diagonal edges.
//     Measured psnr=36.8, p99=13, max=55, <=24 over 99.17% of pixels.
//   - glyph_latin_{24,48}px: SDF text (Slice 5) reconstructs coverage from the
//     binary-threshold signed-distance field via smoothstep, so edge pixels
//     differ from the oracle's exact per-pixel coverage. The 99th percentile
//     stays within the Q1 bound (p99 6-8); the outliers are the ~1.5% of
//     edge-band pixels whose alpha differs by up to ~half (max ~127/255). This
//     is the SDF-vs-exact-coverage model difference, not a silent widening; it
//     is what makes large text crisp and scale-invariant. Measured: 24px
//     psnr=34.1 p99=6 max=127 over 98.57%; 48px psnr=33.7 p99=8 max=127 over
//     98.45%.
//   - gradient_rotated: a rotated rect filled with a linear gradient; the GPU
//     analytic coverage-AA (Q8 amendment) vs the oracle's exact polygon-area
//     coverage on the diagonal edges, the same model difference as
//     solid_rect_rotated_45. Measured psnr=37.0, p99=15, max=55, <=24 over
//     99.17% of pixels (differences confined to the 1px edge band).
var featureTolerances = map[string]equivalence.EquivalenceTolerance{
	"solid_rect_rotated_45": {
		MinPSNR:        36,
		P99Diff:        15,
		MaxDiff:        80,
		WithinFraction: 0.99,
	},
	"gradient_rotated": {
		MinPSNR:        36,
		P99Diff:        15,
		MaxDiff:        80,
		WithinFraction: 0.99,
	},
	"glyph_latin_24px": {
		MinPSNR:        34,
		P99Diff:        8,
		MaxDiff:        130,
		WithinFraction: 0.985,
	},
	"glyph_latin_48px": {
		MinPSNR:        33.5,
		P99Diff:        8,
		MaxDiff:        130,
		WithinFraction: 0.984,
	},
}

func TestCorpusEquivalence(t *testing.T) {
	requireVulkanRaster(t)
	defer func() {
		_ = vulkan.Shutdown()
	}()
	reg := fontdata.TestFontRegistry(t)

	for _, fx := range corpus.All(reg) {
		t.Run(fx.Name(), func(t *testing.T) {
			if reason, ok := deferredFixtures[fx.Name()]; ok {
				t.Skipf("deferred: %s", reason)
			}
			tol, ok := featureTolerances[fx.Name()]
			if !ok {
				tol = equivalence.DefaultTolerance()
			}
			report, err := equivalence.CompareFixture(fx, tol)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if !report.Passed {
				writeDiffArtifacts(t, fx, report)
				t.Fatalf("equivalence failed: %s", report.String())
			}
		})
	}
}

// writeDiffArtifacts persists the software and GPU renders as PNGs to the temp
// dir so a failing fixture can be inspected.
func writeDiffArtifacts(t *testing.T, fx equivalence.FrameFixture, report equivalence.DiffReport) {
	t.Helper()
	width, height := fx.Size()
	dir := t.TempDir()
	write := func(name string, pixels []byte) {
		if len(pixels) != width*height*4 {
			return
		}
		img := image.NewNRGBA(image.Rect(0, 0, width, height))
		copy(img.Pix, pixels)
		path := filepath.Join(dir, name)
		if f, err := os.Create(path); err == nil {
			_ = png.Encode(f, img)
			_ = f.Close()
			t.Logf("%s: %s", name, path)
		}
	}
	if soft, err := equivalence.RenderSoftware(fx.Frame(), width, height); err == nil {
		write(fx.Name()+"_software.png", soft)
	}
	if gpu, err := equivalence.RenderVulkan(fx.Frame(), width, height); err == nil {
		write(fx.Name()+"_gpu.png", gpu)
	}
	t.Logf("report: %s", report.String())
}

// TestCorpusEquivalence_NegativeControl proves the equivalence harness catches a
// real shader regression through the actual shader toolchain (NFR-1). The RG
// channels are swapped in the fragment shader (solid_swapped.frag, built by
// glslc -> SPIR-V -> a pipeline variant selected by a test-only FFI), not by
// post-processing the readback bytes: a regression that produced subtly-wrong
// but non-channel-swapped output would otherwise slip through.
func TestCorpusEquivalence_NegativeControl(t *testing.T) {
	requireVulkanRaster(t)
	reg := fontdata.TestFontRegistry(t)
	fixtures := corpus.All(reg)
	if len(fixtures) == 0 {
		t.Fatal("corpus is empty")
	}

	target := fixtures[0]
	width, height := target.Size()
	soft, err := equivalence.RenderSoftware(target.Frame(), width, height)
	if err != nil {
		t.Fatalf("software render: %v", err)
	}
	gpu, err := equivalence.RenderVulkan(target.Frame(), width, height)
	if err != nil {
		t.Fatalf("gpu render: %v", err)
	}

	// Baseline must pass.
	baseline := equivalence.Compare(soft, gpu, width, height, equivalence.DefaultTolerance())
	if !baseline.Passed {
		t.Fatalf("baseline fixture %s failed: %s", target.Name(), baseline.String())
	}

	// Render through the RG-swapped fragment shader.
	if err := vulkan.ForceSwappedRendering(true); err != nil {
		t.Fatalf("enable swapped shader: %v", err)
	}
	defer func() { _ = vulkan.ForceSwappedRendering(false) }()
	swappedGPU, err := equivalence.RenderVulkan(target.Frame(), width, height)
	if err != nil {
		t.Fatalf("swapped shader render: %v", err)
	}

	// The swapped shader must actually change the output; otherwise the control
	// is void (e.g. the variant was never selected).
	delta := equivalence.Compare(gpu, swappedGPU, width, height, equivalence.DefaultTolerance())
	if delta.Passed {
		t.Fatalf("the RG-swapped shader produced a byte-identical render; the negative control is void")
	}

	// The harness must fail on the shader-level channel swap.
	report := equivalence.Compare(soft, swappedGPU, width, height, equivalence.DefaultTolerance())
	if report.Passed {
		t.Fatalf("RG swap in the fragment shader must fail equivalence, got: %s", report.String())
	}
}

// gpuRenderedCommands are the wire commands the current GPU pipeline
// (Slices 3-8) handles end-to-end (render or state). Every other wire command
// must be explicitly deferred in deferredWireCommands.
var gpuRenderedCommands = map[string]bool{
	"FillRect":           true,
	"StrokeRect":         true,
	"FillPath":           true,
	"StrokePath":         true,
	"DrawPolyline":       true,
	"DrawPoints":         true,
	"DrawSelectionRects": true,
	"DrawImage":          true,
	"DrawGlyphRun":       true,
	"DrawBlurredShadow":  true,
	"PushTransform":      true,
	"PopTransform":       true,
	"PushClipRect":       true,
	"PopClip":            true,
	"PushOpacity":        true,
	"PopOpacity":         true,
}

// deferredWireCommands documents, per wire command the current pipeline cannot
// render, the slice that adds it. This is the "deferred, not forgotten"
// contract: a covered-but-not-rendered command must be listed here, and a
// listed command must have a fixture.
var deferredWireCommands = map[string]string{
	"DrawTexture": "backend-specific texture handles; rendered by TestDrawTexture_Rendered",
}

// TestCorpusCoversEveryGeometryCommand guards against a fixture set that stops
// exercising a command: every drawable opcode in the v2 wire surface must be
// represented by at least one fixture, and every command the GPU pipeline does
// not yet render must be explicitly deferred (with its target slice) rather
// than silently absent.
func TestCorpusCoversEveryGeometryCommand(t *testing.T) {
	reg := fontdata.TestFontRegistry(t)
	fixtures := corpus.All(reg)

	seen := make(map[string]bool)
	for _, fx := range fixtures {
		frame := fx.Frame()
		for _, b := range frame.RenderBatchs {
			for _, cmd := range b.Commands.Commands {
				seen[cmdName(cmd)] = true
			}
		}
	}
	for _, required := range []string{
		"FillRect", "StrokeRect", "FillPath", "StrokePath",
		"DrawPolyline", "DrawPoints", "DrawSelectionRects",
		"PushTransform", "PopTransform", "PushClipRect", "PopClip",
		"PushOpacity", "PopOpacity", "DrawGlyphRun", "DrawImage",
		"DrawTexture", "DrawBlurredShadow",
	} {
		if !seen[required] {
			t.Errorf("corpus does not cover %s", required)
		}
	}

	// A covered command must be either GPU-rendered or explicitly deferred.
	for cmd := range seen {
		if gpuRenderedCommands[cmd] {
			continue
		}
		if _, ok := deferredWireCommands[cmd]; !ok {
			t.Errorf("%s is covered but neither rendered by the GPU pipeline nor explicitly deferred; add it to deferredWireCommands", cmd)
		}
	}

	// A deferred command must still have wire-level coverage.
	for cmd := range deferredWireCommands {
		if !seen[cmd] {
			t.Errorf("deferred command %s has no corpus fixture", cmd)
		}
	}
}

func cmdName(cmd gfx.Command) string {
	switch cmd.(type) {
	case gfx.FillRect:
		return "FillRect"
	case gfx.StrokeRect:
		return "StrokeRect"
	case gfx.FillPath:
		return "FillPath"
	case gfx.StrokePath:
		return "StrokePath"
	case gfx.DrawPolyline:
		return "DrawPolyline"
	case gfx.DrawPoints:
		return "DrawPoints"
	case gfx.DrawSelectionRects:
		return "DrawSelectionRects"
	case gfx.PushTransform:
		return "PushTransform"
	case gfx.PopTransform:
		return "PopTransform"
	case gfx.PushClipRect:
		return "PushClipRect"
	case gfx.PopClip:
		return "PopClip"
	case gfx.PushOpacity:
		return "PushOpacity"
	case gfx.PopOpacity:
		return "PopOpacity"
	case gfx.DrawGlyphRun:
		return "DrawGlyphRun"
	case gfx.DrawImage:
		return "DrawImage"
	case gfx.DrawTexture:
		return "DrawTexture"
	case gfx.DrawBlurredShadow:
		return "DrawBlurredShadow"
	case gfx.BeginRenderBatch:
		return "BeginRenderBatch"
	case gfx.EndRenderBatch:
		return "EndRenderBatch"
	default:
		return fmt.Sprintf("%T", cmd)
	}
}
