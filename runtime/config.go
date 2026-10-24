package runtime

import (
	goruntime "runtime"

	"sync"
	"time"

	"codeburg.org/lexbit/lurpicui/icon"

	"codeburg.org/lexbit/lurpicui/assets"
	"codeburg.org/lexbit/lurpicui/diagnostics"
	"codeburg.org/lexbit/lurpicui/drive/locate"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/input"
	"codeburg.org/lexbit/lurpicui/internal/log"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/projection"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
)

// DiagnosticsHook receives per-frame diagnostic stats.
type DiagnosticsHook interface {
	OnFrame(diagnostics.FrameStats)
}

// backendFallbackSink is an optional capability a DiagnosticsHook implementer
// may add (structurally) to receive GPU→software backend-fallback events
// (FR-12/Q9). The runtime type-asserts its diagnostics hook to this interface,
// mirroring the poisoningSink pattern — the DiagnosticsHook interface itself is
// not widened.
type backendFallbackSink interface {
	OnBackendFallback(diagnostics.BackendFallback)
}

// Config configures the runtime core.
type Config struct {
	TargetFPS       int
	GestureConfig   input.GestureConfig
	WorkerCount     int
	FontRegistry    *text.FontRegistry
	IconResolver    icon.Resolver
	AssetManager    assets.Manager
	AssetRegistry   *assets.AssetRegistryStore
	CommandRegistry *CommandRegistry
	LayerRegistry   *layout.LayerRegistry
	WindowBindings  map[string]platform.Window
	ThemeResolver   *theme.ThemeResolver
	ContentScale    float32
	Logger          log.Logger
	DiagnosticsHook DiagnosticsHook

	// Drive enables the Lurpic Drive Protocol seam: the runtime installs a
	// drive sink whose inbox is drained at the head of phase 2 on the
	// runtime thread (QP-1/QP-6). Disabled by default (FR-4): no socket, no
	// sink, one nil check per frame.
	Drive DriveRuntimeConfig

	// DescribeMark adapts facet implementations to the drive locator's
	// metadata view (kind/role/name/key). Injected by the app layer because
	// the canonical adapters live in the marks package, which imports this
	// one (see drive/describe). Nil disables kind/role/name/key matching —
	// only TestID and coords resolve.
	DescribeMark func(impl facet.FacetImpl) locate.Meta

	// AssetDiagnosticsEnabled controls whether asset-path events (mount,
	// extract, stream, evict) are logged through the runtime logger.
	// When false (default), asset diagnostics are collected in FrameStats
	// but no per-event log output is produced. Set via Config or the
	// LURPIC_ASSET_DIAGNOSTICS environment variable ("true" / "1").
	AssetDiagnosticsEnabled bool

	// AssetsResidencyMode selects the GPU residency strategy for decoded
	// assets: "auto" (GPU-capable backends get GPU residency), "cpu", or "gpu".
	AssetsResidencyMode string
	// AssetCPUBudgetMB is the cap for decoded CPU LOD cache.
	AssetCPUBudgetMB int64
	// AssetGPUBudgetMB is the cap for GPU-resident textures.
	AssetGPUBudgetMB int64
	// AssetUploadBudgetKBPerFrame is the per-frame ceiling for GPU uploads.
	AssetUploadBudgetKBPerFrame int

	// RecoveryDisabled disables per-facet panic quarantine. When false (the
	// default), a panicking facet callback quarantines the facet and its
	// subtree, and the run continues with the rest of the tree unaffected.
	// When true, the panic is re-raised after the diagnostic is captured
	// (poisonReports is still populated with the stack), so tests and
	// benchmarks see the original panic with attribution.
	RecoveryDisabled bool

	// SoftwareBackendFactory builds a software render backend bound to a
	// surface for the GPU-fatal fallback (Q9/FR-12). Owned by the app layer,
	// which also owns backend construction; the runtime cannot import the
	// concrete software package (it would create a test-only import cycle).
	// When nil, a GPU-fatal submit error surfaces as a fatal render error
	// instead of falling back.
	SoftwareBackendFactory func(surface render.Surface) (render.Backend, error)
}

// DriveRuntimeConfig configures the runtime's drive integration. The socket,
// token, and grants live in the app layer (app.DriveConfig); the runtime
// only owns execution-at-a-cut.
type DriveRuntimeConfig struct {
	Enabled bool
}

// DefaultConfig returns a valid runtime configuration.
func DefaultConfig() Config {
	workers := goruntime.NumCPU() - 1
	if workers < 1 {
		workers = 1
	}
	reg, _ := text.NewFontRegistry()
	return Config{
		TargetFPS:     60,
		GestureConfig: input.DefaultGestureConfig(),
		WorkerCount:   workers,
		FontRegistry:  reg,

		AssetsResidencyMode:         "auto",
		AssetCPUBudgetMB:            256,
		AssetGPUBudgetMB:            192,
		AssetUploadBudgetKBPerFrame: 4096,
	}
}

// wallNow is the runtime package's wall-clock source (Debt-4): the single
// read point beside the Clock's injected source. Engine code reads time
// through the Clock, never directly.
var wallNow = time.Now

// FrameTimer controls frame pacing.
type FrameTimer struct {
	targetFPS    int
	targetPeriod time.Duration
	lastFrame    time.Time
	requestCh    chan struct{}
	vsyncCh      chan int64 // receives vsync frameTimeNanos from platform
	// nowFn is the pacing time source (Debt-4): the runtime clock, so pacing
	// reads the same seam as every other engine time consumer. Nil means wall
	// time (FrameTimer used standalone, e.g. in tests).
	nowFn func() time.Time
	mu    sync.Mutex
}

// SetTimeSource installs the pacing time source. Called once by the runtime
// constructor with the clock.
func (t *FrameTimer) SetTimeSource(fn func() time.Time) {
	t.mu.Lock()
	t.nowFn = fn
	t.mu.Unlock()
}

func (t *FrameTimer) now() time.Time {
	t.mu.Lock()
	fn := t.nowFn
	t.mu.Unlock()
	if fn != nil {
		return fn()
	}
	return wallNow()
}

// NewFrameTimer constructs a timer for the given target FPS.
func NewFrameTimer(targetFPS int) *FrameTimer {
	if targetFPS <= 0 {
		targetFPS = 60
	}
	return &FrameTimer{
		targetFPS:    targetFPS,
		targetPeriod: time.Second / time.Duration(targetFPS),
		requestCh:    make(chan struct{}, 1),
		vsyncCh:      make(chan int64, 1),
	}
}

// Vsync delivers a vsync timestamp to the timer for alignment. Called from
// the runtime's event handler when a platform VsyncEvent arrives.
func (t *FrameTimer) Vsync(frameTimeNanos int64) {
	if runtimeTraceActive() {
		runtimeTracef("FrameTimer.Vsync ts=%d pending=%d", frameTimeNanos, len(t.vsyncCh))
	}
	select {
	case t.vsyncCh <- frameTimeNanos:
	default:
	}
}

// Wait blocks until the next frame should begin. It prefers vsync timing
// from the platform when available, falling back to timer-based pacing.
func (t *FrameTimer) Wait() time.Time {

	if t == nil {
		return t.now()
	}
	if runtimeTraceActive() {
		runtimeTracef("FrameTimer.Wait enter last=%v", t.lastFrame.IsZero())
	}

	// Try to receive a vsync timestamp first (non-blocking).
	select {
	case frameTimeNanos := <-t.vsyncCh:
		now := time.Unix(0, frameTimeNanos)
		t.mu.Lock()
		t.lastFrame = now
		t.mu.Unlock()
		if runtimeTraceActive() {
			runtimeTracef("FrameTimer.Wait used-vsync now=%s", now.Format(time.RFC3339Nano))
		}
		return now
	default:
	}

	// Fall back to timer-based pacing.
	tick := t.now()
	t.mu.Lock()
	last := t.lastFrame
	t.mu.Unlock()
	if last.IsZero() {
		t.mu.Lock()
		t.lastFrame = tick
		t.mu.Unlock()
		if runtimeTraceActive() {
			runtimeTracef("FrameTimer.Wait first-tick now=%s", tick.Format(time.RFC3339Nano))
		}
		return tick
	}
	next := last.Add(t.targetPeriod)
	if delay := time.Until(next); delay > 0 {
		select {
		case <-t.requestCh:
		case frameTimeNanos := <-t.vsyncCh:
			now := time.Unix(0, frameTimeNanos)
			t.mu.Lock()
			t.lastFrame = now
			t.mu.Unlock()
			if runtimeTraceActive() {
				runtimeTracef("FrameTimer.Wait early-vsync now=%s", now.Format(time.RFC3339Nano))
			}
			return now
		case <-time.After(delay):
		}
	} else {
		select {
		case <-t.requestCh:
		case frameTimeNanos := <-t.vsyncCh:
			now := time.Unix(0, frameTimeNanos)
			t.mu.Lock()
			t.lastFrame = now
			t.mu.Unlock()
			if runtimeTraceActive() {
				runtimeTracef("FrameTimer.Wait late-vsync now=%s", now.Format(time.RFC3339Nano))
			}
			return now
		default:
		}
	}
	now := t.now()
	t.mu.Lock()
	t.lastFrame = now
	t.mu.Unlock()
	if runtimeTraceActive() {
		runtimeTracef("FrameTimer.Wait fallback now=%s", now.Format(time.RFC3339Nano))
	}
	return now
}

// RequestFrame wakes the timer for an immediate frame if no request is pending.
func (t *FrameTimer) RequestFrame() {

	if runtimeTraceActive() {
		runtimeTracef("FrameTimer.RequestFrame pending=%d", len(t.requestCh))
	}
	select {
	case t.requestCh <- struct{}{}:
	default:
	}
}

// FrameInfo constructs projection timing info.
func (t *FrameTimer) FrameInfo(n uint64, now time.Time) projection.FrameInfo {
	t.mu.Lock()
	last := t.lastFrame
	t.mu.Unlock()
	var delta time.Duration
	if !last.IsZero() {
		delta = now.Sub(last)
	}
	return projection.FrameInfo{Number: n, DeltaTime: delta, WallTime: now}
}
