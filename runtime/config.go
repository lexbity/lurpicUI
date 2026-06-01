package runtime

import (
	goruntime "runtime"
	"sync"
	"time"

	"codeburg.org/lexbit/lurpicui/assets"
	"codeburg.org/lexbit/lurpicui/diagnostics"
	"codeburg.org/lexbit/lurpicui/input"
	"codeburg.org/lexbit/lurpicui/internal/log"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/projection"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
)

// DiagnosticsHook receives per-frame diagnostic stats.
type DiagnosticsHook interface {
	OnFrame(diagnostics.FrameStats)
}

// Config configures the runtime core.
type Config struct {
	TargetFPS       int
	GestureConfig   input.GestureConfig
	WorkerCount     int
	FontRegistry    *text.FontRegistry
	IconResolver    IconResolver
	AssetManager    assets.Manager
	AssetRegistry   *assets.AssetRegistryStore
	CommandRegistry *CommandRegistry
	LayerRegistry   *layout.LayerRegistry
	WindowBindings  map[string]platform.Window
	ThemeResolver   *theme.ThemeResolver
	ContentScale    float32
	Logger          log.Logger
	DiagnosticsHook DiagnosticsHook

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

// FrameTimer controls frame pacing.
type FrameTimer struct {
	targetFPS    int
	targetPeriod time.Duration
	lastFrame    time.Time
	requestCh    chan struct{}
	vsyncCh      chan int64 // receives vsync frameTimeNanos from platform
	mu           sync.Mutex
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
		return time.Now()
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
	tick := time.Now()
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
	now := time.Now()
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
