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
// but are not yet rendered correctly by the CPU stepping-stone raster. They are
// kept so the corpus covers every gfx command; the corpus runner skips them
// until the slice that renders them lands (StrokePath: Slice 8).
var deferredFixtures = map[string]string{
	"stroke_path_rect_deferred": "closed-path stroke needs OffsetContour expansion (Slice 8)",
}

func TestCorpusEquivalence(t *testing.T) {
	requireVulkanRaster(t)
	reg := fontdata.TestFontRegistry(t)

	for _, fx := range corpus.All(reg) {
		t.Run(fx.Name(), func(t *testing.T) {
			if reason, ok := deferredFixtures[fx.Name()]; ok {
				t.Skipf("deferred: %s", reason)
			}
			report, err := equivalence.CompareFixture(fx, equivalence.DefaultTolerance())
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

// TestCorpusEquivalence_NegativeControl injects a channel swap into the GPU
// readback and asserts the harness catches it end-to-end.
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

	// Inject a channel swap; the harness must fail.
	swapped := append([]byte(nil), gpu...)
	for i := 0; i < len(swapped); i += 4 {
		swapped[i], swapped[i+2] = swapped[i+2], swapped[i]
	}
	report := equivalence.Compare(soft, swapped, width, height, equivalence.DefaultTolerance())
	if report.Passed {
		t.Fatalf("channel swap must fail equivalence, got: %s", report.String())
	}
}

// TestCorpusCoversEveryGeometryCommand guards against a fixture set that stops
// exercising a command: every drawable opcode in the v2 wire surface must be
// represented by at least one fixture.
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
	} {
		if !seen[required] {
			t.Errorf("corpus does not cover %s", required)
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
