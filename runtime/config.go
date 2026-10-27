package runtime

import (
	goruntime "runtime"
	"sync"
	"time"

	"codeburg.org/lexbit/lurpicui/drive"

	"codeburg.org/lexbit/lurpicui/icon"

	"codeburg.org/lexbit/lurpicui/assets"
	"codeburg.org/lexbit/lurpicui/diagnostics"
	"codeburg.org/lexbit/lurpicui/drive/locate"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/input"
	"codeburg.org/lexbit/lurpicui/internal/log"
	"codeburg.org/lexbit/lurpicui/internal/membudget"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/projection"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/store"
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
	// StoreScope owns every reactive store reachable from the root application.
	// It must be created before the root builder and is closed at shutdown.
	StoreScope    *store.Scope
	StoreIdentity *store.RuntimeIdentity
	TargetFPS     int
	Clock         ClockConfig
	PreparedClock *PreparedClock
	GestureConfig input.GestureConfig
	WorkerCount   int
	// ShutdownGrace is the supervisor's bounded wait before falling back to
	// frameMu-protected teardown when the loop is wedged (D7 rule 3). Default 5s.
	ShutdownGrace   time.Duration
	FontRegistry    *text.FontRegistry
	TextService     *text.Service
	Resources       ResourcePolicy
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

	// ReadExports declares app-wide read capabilities before the runtime
	// contract catalog freezes and the drive listener starts.
	ReadExports []store.ReadExport

	// DescribeMark adapts facet implementations to the drive locator's
	// metadata view (kind/role/name/key). Injected by the app layer because
	// the canonical adapters live in the marks package, which imports this
	// one (see drive/describe). Nil disables kind/role/name/key matching —
	// only TestID and coords resolve.
	DescribeMark func(impl facet.FacetImpl) locate.Meta
	// HostedChildren returns visible mark-owned children. App wiring supplies
	// the canonical capability probe from the marks root package. It is
	// required so runtime identity coverage includes virtualized children.
	HostedChildren func(impl facet.FacetImpl) []facet.FacetImpl

	// AssetDiagnosticsEnabled controls whether asset-path events (mount,
	// extract, stream, evict) are logged through the runtime logger.
	// When false (default), asset diagnostics are collected in FrameStats
	// but no per-event log output is produced. Set via Config or the
	// LURPIC_ASSET_DIAGNOSTICS environment variable ("true" / "1").
	AssetDiagnosticsEnabled bool

	// AssetsResidencyMode selects the GPU residency strategy for decoded
	// assets: "auto" (GPU-capable backends get GPU residency), "cpu", or "gpu".
	AssetsResidencyMode string

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
	// ObservationNodeCapacity and ObservationByteCapacity bound the optional
	// immutable structural-read index. Zero selects the finite defaults.
	ObservationNodeCapacity int
	ObservationByteCapacity int
	// Sessions is the instance session table; the runtime admits commands
	// through its ledgers. Required when Enabled.
	Sessions *drive.SessionTable
	// Stats is the shared drive counter set; a private one when nil.
	Stats *drive.Stats
}

// DefaultConfig returns a valid runtime configuration.
func DefaultConfig() Config {
	profile := membudget.Standard
	if goruntime.GOOS == "android" {
		profile = membudget.Compact
	}
	resources := DefaultResourcePolicy(profile)
	wallet, _ := membudget.NewWallet(resources.Plan.Caps[membudget.Text])
	reg, _ := text.NewFontRegistry(wallet, resources.TextLimits.MaxFontFaces)
	return Config{
		TargetFPS:     60,
		GestureConfig: input.DefaultGestureConfig(),
		WorkerCount:   ProfileWorkerCounts(profile).GeneralWorkers,
		FontRegistry:  reg,
		Resources:     resources,

		AssetsResidencyMode: "auto",
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
	// paceNow is monotonic wall time. It must never be the logical engine
	// clock because a fixed virtual epoch would make pacing deadlines stale.
	paceNow func() time.Time
	mu      sync.Mutex
}

// SetPacingClock installs the monotonic wall clock used only for pacing.
func (t *FrameTimer) SetPacingClock(fn func() time.Time) {
	t.mu.Lock()
	t.paceNow = fn
	t.mu.Unlock()
}

func (t *FrameTimer) now() time.Time {
	t.mu.Lock()
	fn := t.paceNow
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
	now, _ := t.WaitWithWake(nil, nil, nil, nil)
	return now
}

// WaitWithWake preserves the pacing deadline while allowing the runtime owner
// to drain typed platform events, including the VsyncEvent that satisfies this
// wait. The callback always runs on the caller's thread. The shutdown channel,
// if non-nil, is an additional wake source that triggers immediate return with
// false when closed (D7 rule 1: all park points wake on shutdownCh).
func (t *FrameTimer) WaitWithWake(wake <-chan struct{}, onWake func(), frameEligible func() bool, shutdown <-chan struct{}) (time.Time, bool) {

	if t == nil {
		return time.Time{}, false
	}
	// If shutdown is already closed, return immediately.
	if shutdown != nil {
		select {
		case <-shutdown:
			return time.Time{}, false
		default:
		}
	}
	eligible := func() bool { return frameEligible == nil || frameEligible() }
	if runtimeTraceActive() {
		runtimeTracef("FrameTimer.Wait enter last=%v", t.lastFrame.IsZero())
	}

	first := true
	for {
		select {
		case <-shutdown:
			return time.Time{}, false
		case frameTimeNanos := <-t.vsyncCh:
			if !eligible() {
				return time.Time{}, false
			}
			return t.recordVsync(frameTimeNanos, "vsync"), true
		default:
		}
		tick := t.now()
		t.mu.Lock()
		last := t.lastFrame
		t.mu.Unlock()
		if last.IsZero() {
			if !eligible() {
				return time.Time{}, false
			}
			t.mu.Lock()
			t.lastFrame = tick
			t.mu.Unlock()
			if runtimeTraceActive() {
				runtimeTracef("FrameTimer.Wait first-tick now=%s", tick.Format(time.RFC3339Nano))
			}
			return tick, true
		}
		delay := last.Add(t.targetPeriod).Sub(t.now())
		if delay <= 0 {
			select {
			case <-t.requestCh:
			case <-shutdown:
				return time.Time{}, false
			default:
			}
			// Check shutdown again after the request check
			if shutdown != nil {
				select {
				case <-shutdown:
					return time.Time{}, false
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
			return now, true
		}
		timer := time.NewTimer(delay)
		select {
		case <-shutdown:
			timer.Stop()
			return time.Time{}, false
		case <-t.requestCh:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			now := t.now()
			t.mu.Lock()
			t.lastFrame = now
			t.mu.Unlock()
			return now, true
		case frameTimeNanos := <-t.vsyncCh:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			label := "late-vsync"
			if first {
				label = "early-vsync"
			}
			if !eligible() {
				return time.Time{}, false
			}
			return t.recordVsync(frameTimeNanos, label), true
		case <-wake:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			if onWake != nil {
				onWake()
			}
			if !eligible() {
				return time.Time{}, false
			}
			// Check shutdown after wake
			if shutdown != nil {
				select {
				case <-shutdown:
					return time.Time{}, false
				default:
				}
			}
			first = false
		case <-timer.C:
			if !eligible() {
				return time.Time{}, false
			}
			// Check shutdown after timer fired
			if shutdown != nil {
				select {
				case <-shutdown:
					return time.Time{}, false
				default:
				}
			}
			now := t.now()
			t.mu.Lock()
			t.lastFrame = now
			t.mu.Unlock()
			if runtimeTraceActive() {
				runtimeTracef("FrameTimer.Wait timeout now=%s", now.Format(time.RFC3339Nano))
			}
			return now, true
		}
	}
}

func (t *FrameTimer) recordVsync(frameTimeNanos int64, label string) time.Time {
	now := time.Unix(0, frameTimeNanos)
	t.mu.Lock()
	t.lastFrame = now
	t.mu.Unlock()
	if runtimeTraceActive() {
		runtimeTracef("FrameTimer.Wait %s now=%s", label, now.Format(time.RFC3339Nano))
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

func (t *FrameTimer) idleChannels() (<-chan struct{}, <-chan int64) {
	if t == nil {
		return nil, nil
	}
	return t.requestCh, t.vsyncCh
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
