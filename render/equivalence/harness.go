// Package equivalence renders a corpus of frames against both the software
// backend (the correctness oracle) and the Vulkan GPU backend, and asserts
// perceptual equivalence within a bounded tolerance (PSNR, per-channel
// percentiles, max-diff). Each slice that adds a rendered feature adds corpus
// fixtures that must meet the tolerance.
package equivalence

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/software"
	"codeburg.org/lexbit/lurpicui/render/vulkan"
)

// FrameFixture produces a renderable frame at a fixed canvas size.
type FrameFixture interface {
	Name() string
	Size() (width, height int)
	Frame() *render.Frame
}

// EquivalenceTolerance bounds the perceptual difference between the two
// backends. Calibrated against the CPU analytic AA (software oracle) vs the GPU
// pipeline's coverage AA; see devdocs/notes/vulkan-equivalence-baseline.md.
type EquivalenceTolerance struct {
	// MinPSNR is the minimum PSNR over RGBA in dB.
	MinPSNR float64
	// P99Diff is the 99th percentile of per-channel absolute differences.
	P99Diff float64
	// MaxDiff is the per-pixel max-channel difference that at least
	// WithinFraction of pixels must meet.
	MaxDiff float64
	// WithinFraction is the minimum fraction of pixels whose max-channel
	// difference is <= MaxDiff.
	WithinFraction float64
}

// DefaultTolerance returns the Q1 tolerance: PSNR >= 40 dB, P99 <= 8/255,
// max <= 24/255 over >= 99.5% of pixels.
func DefaultTolerance() EquivalenceTolerance {
	return EquivalenceTolerance{
		MinPSNR:        40,
		P99Diff:        8,
		MaxDiff:        24,
		WithinFraction: 0.995,
	}
}

// DiffReport summarizes a comparison between two renders.
type DiffReport struct {
	Width  int
	Height int
	// PSNR over all RGBA channels, dB.
	PSNR float64
	// P99Diff is the 99th percentile of per-channel absolute differences.
	P99Diff float64
	// MaxDiff is the maximum per-pixel max-channel difference observed.
	MaxDiff float64
	// WithinMaxDiff is the fraction of pixels within MaxDiff.
	WithinMaxDiff float64
	// OutlierCount is the number of pixels exceeding the tolerance MaxDiff.
	OutlierCount int
	Passed       bool
}

func (r DiffReport) String() string {
	return fmt.Sprintf(
		"psnr=%.1fdb p99=%.1f max=%.1f within_max=%.4f outliers=%d passed=%v",
		r.PSNR, r.P99Diff, r.MaxDiff, r.WithinMaxDiff, r.OutlierCount, r.Passed,
	)
}

// Compare diffs two RGBA buffers and reports whether they meet the tolerance.
// The buffers must both be width*height*4 bytes.
func Compare(software, gpu []byte, width, height int, tol EquivalenceTolerance) DiffReport {
	report := DiffReport{Width: width, Height: height}
	if len(software) != width*height*4 || len(gpu) != width*height*4 {
		report.Passed = false
		return report
	}
	psnr, p99, within, maxObserved, outliers := computeDiff(software, gpu, width, height, tol)
	report.PSNR = psnr
	report.P99Diff = p99
	report.WithinMaxDiff = within
	report.MaxDiff = maxObserved
	report.OutlierCount = outliers
	report.Passed = psnr >= tol.MinPSNR && p99 <= tol.P99Diff && within >= tol.WithinFraction
	return report
}

// memSurface is an in-memory render.SoftwareSurface used to capture software
// output as RGBA pixels.
type memSurface struct {
	width  int
	height int
	buf    []byte
}

func (s *memSurface) Size() (int, int) { return s.width, s.height }
func (s *memSurface) Resize(width, height int) {
	s.width = width
	s.height = height
	s.buf = make([]byte, width*height*4)
}
func (s *memSurface) Buffer() []byte                { return s.buf }
func (s *memSurface) Stride() int                   { return s.width * 4 }
func (s *memSurface) Lock() error                   { return nil }
func (s *memSurface) Unlock(dirty []gfx.Rect) error { return nil }

// RenderSoftware renders the frame with the software backend and returns RGBA
// pixels at width*height.
func RenderSoftware(f *render.Frame, width, height int) ([]byte, error) {
	surface := &memSurface{width: width, height: height}
	surface.Resize(width, height)
	r := software.NewSoftwareRenderer()
	if err := r.Initialize(surface); err != nil {
		return nil, fmt.Errorf("equivalence: software init: %w", err)
	}
	defer r.Destroy()
	if err := r.Submit(f); err != nil {
		return nil, fmt.Errorf("equivalence: software submit: %w", err)
	}
	out := make([]byte, width*height*4)
	copy(out, surface.Buffer())
	return out, nil
}

// RenderVulkan encodes the frame to packet v2 and renders it through the GPU
// solid pipeline into an offscreen target, returning RGBA pixels at width*height.
// The Vulkan renderer is initialized lazily (idempotent); the corpus test
// shuts it down once at the end.
func RenderVulkan(f *render.Frame, width, height int) ([]byte, error) {
	if err := vulkan.Init(); err != nil {
		return nil, fmt.Errorf("equivalence: vulkan init: %w", err)
	}
	packet, err := vulkan.EncodeFrame(f)
	if err != nil {
		return nil, fmt.Errorf("equivalence: encode: %w", err)
	}
	out, err := vulkan.SubmitAndReadback(packet, width, height)
	if err != nil {
		return nil, fmt.Errorf("equivalence: readback: %w", err)
	}
	return out, nil
}

// CompareFixture renders a fixture to both backends and returns the report.
// It is used by the corpus test and by per-slice fixture tests.
func CompareFixture(fixture FrameFixture, tol EquivalenceTolerance) (DiffReport, error) {
	width, height := fixture.Size()
	frame := fixture.Frame()
	soft, err := RenderSoftware(frame, width, height)
	if err != nil {
		return DiffReport{}, err
	}
	gpu, err := RenderVulkan(frame, width, height)
	if err != nil {
		return DiffReport{}, err
	}
	return Compare(soft, gpu, width, height, tol), nil
}
