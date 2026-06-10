package testkit

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/diagnostics"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/software"
	"codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/text"
)

// HarnessConfig configures a headless harness.
type HarnessConfig struct {
	Width, Height    int
	Fonts            []text.FontSource
	LayerRegistry    *layout.LayerRegistry
	Backend          render.Backend          // optional; defaults to software renderer
	WritingDirection layout.WritingDirection // empty defaults to LTR
}

// DefaultHarnessConfig returns an 800x600 harness config.
func DefaultHarnessConfig() HarnessConfig {
	return HarnessConfig{Width: 800, Height: 600}
}

// Harness is a lightweight headless test wrapper.
type Harness struct {
	t          testing.TB
	app        *NullApp
	surface    *MemorySurface
	fonts      *text.FontRegistry
	rt         *runtime.Runtime
	FrameCount int
}

// NewHarness creates a headless harness and registers cleanup.
func NewHarness(t testing.TB, config HarnessConfig, root facet.FacetImpl) *Harness {
	t.Helper()
	if config.Width <= 0 {
		config.Width = 800
	}
	if config.Height <= 0 {
		config.Height = 600
	}
	app := NewNullApp(config.Width, config.Height)
	fonts, err := text.NewFontRegistry()
	if err != nil {
		t.Fatalf("font registry: %v", err)
	}
	for _, src := range config.Fonts {
		if src.Path != "" {
			if err := fonts.LoadFontFile(src.Path); err != nil {
				t.Fatalf("load font file: %v", err)
			}
			continue
		}
		if len(src.Data) > 0 {
			if err := fonts.LoadFontBytes(src.Data, src.Name); err != nil {
				t.Fatalf("load font bytes: %v", err)
			}
		}
	}
	win, err := app.NewWindow(platform.WindowOptions{Width: config.Width, Height: config.Height})
	if err != nil {
		t.Fatalf("new window: %v", err)
	}
	window := win.(*NullWindow)
	if config.LayerRegistry == nil {
		t.Fatal("testkit: LayerRegistry is required")
	}
	surface := window.surface
	backend := config.Backend
	if backend == nil {
		backend = software.NewSoftwareRenderer()
	}
	if err := backend.Initialize(surface); err != nil {
		t.Fatalf("initialize backend: %v", err)
	}
	rtcfg := runtime.DefaultConfig()
	rtcfg.FontRegistry = fonts
	rtcfg.LayerRegistry = config.LayerRegistry
	rt, err := runtime.New(rtcfg, app, window, backend, root)
	if err != nil {
		t.Fatalf("runtime: %v", err)
	}
	h := &Harness{t: t, app: app, surface: surface, fonts: fonts, rt: rt}
	t.Cleanup(func() {
		rt.Shutdown()
		app.Destroy()
	})
	return h
}

func (h *Harness) RunFrame() {
	if h == nil {
		return
	}
	if h.rt != nil {
		h.rt.RunOneFrame()
	}
	h.FrameCount++
}

func (h *Harness) RunFrames(n int) {
	for i := 0; i < n; i++ {
		h.RunFrame()
	}
}

func (h *Harness) RunUntil(condition func() bool, maxFrames int) bool {
	for i := 0; i < maxFrames; i++ {
		h.RunFrame()
		if condition != nil && condition() {
			return true
		}
	}
	return condition != nil && condition()
}

func (h *Harness) Surface() *MemorySurface {
	if h == nil {
		return nil
	}
	return h.surface
}

func (h *Harness) InjectEvent(e platform.Event) {
	if h == nil || h.app == nil {
		return
	}
	h.app.InjectEvent(e)
}

func (h *Harness) InjectEvents(events []platform.Event) {
	for _, e := range events {
		h.InjectEvent(e)
	}
}

// Runtime returns the underlying runtime.
func (h *Harness) Runtime() *runtime.Runtime {
	if h == nil {
		return nil
	}
	return h.rt
}

// LastFrameStats returns the most recent runtime frame statistics.
func (h *Harness) LastFrameStats() diagnostics.FrameStats {
	if h == nil || h.rt == nil {
		return diagnostics.FrameStats{}
	}
	return h.rt.LastFrameStats()
}

// SimulateSurfaceLost triggers the surface lost callback registered by the runtime.
// After this, render submits will be skipped until SimulateSurfaceRestore is called.
func (h *Harness) SimulateSurfaceLost() {
	if h == nil || h.app == nil {
		return
	}
	h.app.TriggerSurfaceLost()
}

// SimulateSurfaceRestore triggers the surface created callback, passing the
// current surface so the render backend can reinitialize.
func (h *Harness) SimulateSurfaceRestore() {
	if h == nil || h.app == nil || h.surface == nil {
		return
	}
	h.app.TriggerSurfaceCreated(h.surface)
}

// SimulatePause triggers the pause callback registered by the runtime.
func (h *Harness) SimulatePause() {
	if h == nil || h.app == nil {
		return
	}
	h.app.TriggerPause()
}

// SimulateResume triggers the resume callback registered by the runtime.
func (h *Harness) SimulateResume() {
	if h == nil || h.app == nil {
		return
	}
	h.app.TriggerResume()
}
