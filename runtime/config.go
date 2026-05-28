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
	}
}

// FrameTimer controls frame pacing.
type FrameTimer struct {
	targetFPS    int
	targetPeriod time.Duration
	lastFrame    time.Time
	requestCh    chan struct{}
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
	}
}

// Wait blocks until the next frame should begin.
func (t *FrameTimer) Wait() time.Time {

	tick := time.Now()
	t.mu.Lock()
	last := t.lastFrame
	t.mu.Unlock()
	if last.IsZero() {
		t.mu.Lock()
		t.lastFrame = tick
		t.mu.Unlock()
		return tick
	}
	next := last.Add(t.targetPeriod)
	if delay := time.Until(next); delay > 0 {
		select {
		case <-t.requestCh:
		case <-time.After(delay):
		}
	} else {
		select {
		case <-t.requestCh:
		default:
		}
	}
	now := time.Now()
	t.mu.Lock()
	t.lastFrame = now
	t.mu.Unlock()
	return now
}

// RequestFrame wakes the timer for an immediate frame if no request is pending.
func (t *FrameTimer) RequestFrame() {

	select {
	case t.requestCh <- struct{}{}:
	default:
	}
}

// FrameInfo constructs projection timing info.
func (t *FrameTimer) FrameInfo(n uint64, now time.Time) projection.FrameInfo {
	if t == nil {
		return projection.FrameInfo{Number: n, WallTime: now}
	}
	t.mu.Lock()
	last := t.lastFrame
	t.mu.Unlock()
	var delta time.Duration
	if !last.IsZero() {
		delta = now.Sub(last)
	}
	return projection.FrameInfo{Number: n, DeltaTime: delta, WallTime: now}
}
