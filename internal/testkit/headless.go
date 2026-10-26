package testkit

import (
	"sync"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/diagnostics"
	"codeburg.org/lexbit/lurpicui/drive"
	drivedesc "codeburg.org/lexbit/lurpicui/drive/describe"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/internal/membudget"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/software"
	"codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
)

// HarnessConfig configures a headless harness.
type HarnessConfig struct {
	Width, Height    int
	Clock            runtime.ClockConfig
	Fonts            []text.FontSource
	LayerRegistry    *layout.LayerRegistry
	ThemeResolver    *theme.ThemeResolver    // optional; the anchored-layer recipe resolver
	Backend          render.Backend          // optional; defaults to software renderer
	WritingDirection layout.WritingDirection // empty defaults to LTR
	// Drive installs the runtime drive port in-process (no socket),
	// enabling the DriveHarness and direct drive.RuntimePort access.
	Drive bool
	// DriveSessions is the session table the runtime admits commands
	// through; the DriveHarness installs one when unset.
	DriveSessions *drive.SessionTable
	// DriveStats is the shared drive counter set; a private one when nil.
	DriveStats *drive.Stats
	// CommandRegistry wires the app's command registry (the cmd.execute
	// surface). Nil means no registry configured.
	CommandRegistry *runtime.CommandRegistry
}

// DefaultHarnessConfig returns an 800x600 harness config.
func DefaultHarnessConfig() HarnessConfig {
	return HarnessConfig{Width: 800, Height: 600}
}

// StandardLayerRegistry returns the frozen standard layer registry, failing the
// test if it cannot be constructed. HarnessConfig requires a non-nil
// LayerRegistry; this is the canonical source for integration and golden tests.
func StandardLayerRegistry(t testing.TB) *layout.LayerRegistry {
	t.Helper()
	reg, err := layout.StandardLayerRegistry()
	if err != nil {
		t.Fatalf("standard layer registry: %v", err)
	}
	return reg
}

// StandardHarnessConfig returns a HarnessConfig for integration and golden
// tests: the given size, the standard layer registry, and the canonical test
// font. Most marks render text, which requires a font registry; this is the
// canonical way to build a config that produces a fully rendered frame.
func StandardHarnessConfig(t testing.TB, width, height int) HarnessConfig {
	t.Helper()
	return HarnessConfig{
		Width:         width,
		Height:        height,
		LayerRegistry: StandardLayerRegistry(t),
		Fonts:         []text.FontSource{{Name: "noto-sans-regular", Data: TestFontBytes()}},
	}
}

// NewStandardHarness constructs a Harness from StandardHarnessConfig for the
// given size and root. It is the canonical constructor for interaction and
// golden tests that render text-bearing marks.
func NewStandardHarness(t testing.TB, scope *store.Scope, width, height int, root facet.FacetImpl) *Harness {
	t.Helper()
	return NewHarness(t, scope, StandardHarnessConfig(t, width, height), root)
}

// Harness is a lightweight headless test wrapper.
type Harness struct {
	t          testing.TB
	app        *NullApp
	surface    *MemorySurface
	fonts      *text.FontRegistry
	rt         *runtime.Runtime
	FrameCount int
	// driveSessions is the session table backing the in-process drive seam
	// (nil when Drive is disabled).
	driveSessions *drive.SessionTable
	driveMu       sync.Mutex
	driveClient   *harnessDriveClient
	frameObserver func(*runtime.Runtime, *MemorySurface)
	observedFrame uint64
	// frameClock is the synthetic frame-time source: RunFrame seeds the
	// runtime clock from it before each frame, so frame dt is fully under
	// the test's control (zero by default — headless frames carry no
	// spontaneous elapsed time; a feed or cadence only ticks when the test
	// asks for it via SetFramePeriod).
	frameClock  time.Time
	framePeriod time.Duration
}

// SetFramePeriod makes each subsequent RunFrame advance the harness clock by
// d, so every frame observes dt = d. Zero (the default) keeps dt at 0.
func (h *Harness) SetFramePeriod(d time.Duration) {
	if h == nil {
		return
	}
	h.framePeriod = d
}

// NewHarness creates a headless harness and registers cleanup.
func NewHarness(t testing.TB, scope *store.Scope, config HarnessConfig, root facet.FacetImpl) *Harness {
	t.Helper()
	if config.Width <= 0 {
		config.Width = 800
	}
	if config.Height <= 0 {
		config.Height = 600
	}
	app := NewNullApp(config.Width, config.Height)
	resources := runtime.DefaultResourcePolicy(membudget.Standard)
	textWallet, err := membudget.NewWallet(resources.Plan.Caps[membudget.Text])
	if err != nil {
		t.Fatalf("text wallet: %v", err)
	}
	fonts, err := text.NewFontRegistry(textWallet, resources.TextLimits.MaxFontFaces)
	if err != nil {
		t.Fatalf("font registry: %v", err)
	}
	t.Cleanup(func() { _ = fonts.Close() })
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
	if scope == nil {
		t.Fatal("testkit: store scope is required")
	}
	t.Cleanup(scope.Close)
	rtcfg.StoreScope = scope
	rtcfg.StoreIdentity = store.NewRuntimeIdentity()
	rtcfg.Clock = config.Clock
	rtcfg.FontRegistry = fonts
	rtcfg.LayerRegistry = config.LayerRegistry
	rtcfg.ThemeResolver = config.ThemeResolver
	rtcfg.Drive.Enabled = config.Drive
	if config.Drive && config.DriveSessions == nil {
		inst, err := drive.NewInstance()
		if err != nil {
			t.Fatalf("drive instance: %v", err)
		}
		config.DriveSessions = drive.NewSessionTable(inst)
	}
	rtcfg.Drive.Sessions = config.DriveSessions
	rtcfg.Drive.Stats = config.DriveStats
	rtcfg.DescribeMark = drivedesc.Describe
	rtcfg.HostedChildren = marks.HostedChildrenFor
	rtcfg.CommandRegistry = config.CommandRegistry
	rt, err := runtime.New(rtcfg, app, window, backend, root)
	if err != nil {
		t.Fatalf("runtime: %v", err)
	}
	h := &Harness{t: t, app: app, surface: surface, fonts: fonts, rt: rt, driveSessions: config.DriveSessions}
	t.Cleanup(func() {
		rt.Shutdown()
		h.driveMu.Lock()
		if h.driveClient != nil && h.driveSessions != nil {
			h.driveSessions.Revoke(h.driveClient.sess.ID)
			h.driveClient = nil
		}
		h.driveMu.Unlock()
		app.Destroy()
	})
	return h
}

func (h *Harness) RunFrame() {
	if h == nil {
		return
	}
	if h.rt != nil {
		if h.frameClock.IsZero() {
			// Fixed epoch: with a zero period every frame has dt = 0.
			h.frameClock = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		}
		h.rt.RunOneFrameAt(h.frameClock)
		h.frameClock = h.frameClock.Add(h.framePeriod)
		if h.frameObserver != nil {
			frame := h.rt.LastFrameStats().FrameNumber
			if frame != 0 && frame != h.observedFrame {
				h.observedFrame = frame
				h.frameObserver(h.rt, h.surface)
			}
		}
	}
	h.FrameCount++
}

// SetFrameObserver installs a synchronous observer for each newly completed
// runtime frame. It is intended for test evidence installed before input.
func (h *Harness) SetFrameObserver(observer func(*runtime.Runtime, *MemorySurface)) {
	if h == nil {
		return
	}
	h.frameObserver = observer
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
