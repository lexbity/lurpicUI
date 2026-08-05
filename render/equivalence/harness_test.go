package equivalence

import (
	"image/color"
	"math"
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/vulkan"
)

type simpleRect struct{}

func (simpleRect) Name() string     { return "simple_rect" }
func (simpleRect) Size() (int, int) { return 16, 16 }
func (simpleRect) Frame() *render.Frame {
	return &render.Frame{
		RenderBatchs: []render.RenderBatch{{
			ID:      1,
			Bounds:  gfx.RectFromXYWH(0, 0, 16, 16),
			Opacity: 1,
			Commands: gfx.CommandList{Commands: []gfx.Command{
				gfx.FillRect{Rect: gfx.RectFromXYWH(4, 4, 8, 8), Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(255, 0, 0, 255))},
			}},
		}},
	}
}

func simpleRectFixture() FrameFixture { return simpleRect{} }

func requireVulkanRaster(t *testing.T) {
	t.Helper()
	if _, err := vulkan.Version(); err != nil {
		t.Skipf("Rust raster renderer unavailable: %v", err)
	}
}

func filled(width, height int, fill color.RGBA) []byte {
	out := make([]byte, width*height*4)
	for i := 0; i < width*height; i++ {
		out[i*4] = fill.R
		out[i*4+1] = fill.G
		out[i*4+2] = fill.B
		out[i*4+3] = fill.A
	}
	return out
}

func TestCompareIdenticalBuffersPass(t *testing.T) {
	w, h := 16, 16
	buf := filled(w, h, color.RGBA{R: 128, G: 200, B: 60, A: 255})
	report := Compare(buf, buf, w, h, DefaultTolerance())
	if !report.Passed {
		t.Fatalf("identical buffers must pass: %s", report.String())
	}
	if math.IsInf(report.PSNR, 1) == false {
		t.Fatalf("expected infinite PSNR for identical buffers, got %v", report.PSNR)
	}
}

func TestCompareOneLsbDriftPasses(t *testing.T) {
	w, h := 16, 16
	a := filled(w, h, color.RGBA{R: 100, G: 150, B: 200, A: 255})
	b := filled(w, h, color.RGBA{R: 101, G: 150, B: 200, A: 255})
	report := Compare(a, b, w, h, DefaultTolerance())
	if !report.Passed {
		t.Fatalf("a 1 LSB channel drift must pass: %s", report.String())
	}
	if report.P99Diff != 1 {
		t.Fatalf("P99 = %v, want 1", report.P99Diff)
	}
}

func TestCompareChannelSwapFails(t *testing.T) {
	w, h := 16, 16
	a := filled(w, h, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	b := filled(w, h, color.RGBA{R: 0, G: 0, B: 255, A: 255})
	report := Compare(a, b, w, h, DefaultTolerance())
	if report.Passed {
		t.Fatalf("channel swap must fail equivalence: %s", report.String())
	}
}

func TestCompareAlphaBugFails(t *testing.T) {
	w, h := 16, 16
	a := filled(w, h, color.RGBA{R: 200, G: 120, B: 60, A: 255})
	b := filled(w, h, color.RGBA{R: 0, G: 0, B: 0, A: 0})
	report := Compare(a, b, w, h, DefaultTolerance())
	if report.Passed {
		t.Fatalf("an alpha-zeroing bug must fail equivalence: %s", report.String())
	}
}

func TestCompareEdgeOutliersAreTolerated(t *testing.T) {
	// Interior pixels match; a handful of edge pixels differ by more than the
	// max (simulating AA differences on < 0.5% of pixels) must still pass.
	w, h := 64, 64
	a := filled(w, h, color.RGBA{R: 30, G: 80, B: 120, A: 255})
	b := make([]byte, len(a))
	copy(b, a)
	// Corrupt 10 pixels out of 4096 (~0.24%) with a diff just above the max
	// (edge-AA scale) so the outlier allowance applies without cratering PSNR.
	for i := 0; i < 10; i++ {
		off := i * 4
		b[off] = 60 // diff 30 vs base 30
		b[off+3] = 255
	}
	report := Compare(a, b, w, h, DefaultTolerance())
	if !report.Passed {
		t.Fatalf("edge outliers within the 0.5%% allowance must pass: %s", report.String())
	}
}

func TestCompareManyEdgeOutliersFail(t *testing.T) {
	// More than 0.5% of pixels differing beyond the max must fail.
	w, h := 64, 64
	a := filled(w, h, color.RGBA{R: 30, G: 80, B: 120, A: 255})
	b := make([]byte, len(a))
	copy(b, a)
	for i := 0; i < 100; i++ { // ~2.4%
		off := i * 4
		b[off] = 200
		b[off+3] = 255
	}
	report := Compare(a, b, w, h, DefaultTolerance())
	if report.Passed {
		t.Fatalf("outlier fraction above the allowance must fail: %s", report.String())
	}
}

func TestRenderSoftwareCapturesPixels(t *testing.T) {
	fx := simpleRectFixture()
	frame := fx.Frame()
	out, err := RenderSoftware(frame, 16, 16)
	if err != nil {
		t.Fatalf("RenderSoftware: %v", err)
	}
	if len(out) != 16*16*4 {
		t.Fatalf("output size = %d", len(out))
	}
	// Pixel at (8,8) is inside the red rect. Software output is RGBA.
	off := (8*16 + 8) * 4
	if out[off] < 200 || out[off+3] < 200 {
		t.Fatalf("expected red rect pixel, got %v", out[off:off+4])
	}
}

func TestCompareFixtureReportsDiff(t *testing.T) {
	requireVulkanRaster(t)
	fx := simpleRectFixture()
	report, err := CompareFixture(fx, DefaultTolerance())
	if err != nil {
		t.Fatalf("CompareFixture: %v", err)
	}
	if !report.Passed {
		t.Fatalf("self-consistent fixture must pass: %s", report.String())
	}
}
