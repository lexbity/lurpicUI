package runtime

import (
	"errors"
	"time"
)

// ClockMode selects who owns time (QP-5).
type ClockMode uint8

const (
	// ClockWall reads the monotonic system clock; the frame loop is
	// vsync/event-driven.
	ClockWall ClockMode = iota
	// ClockVirtual single-steps: frames render only when a driver advances
	// time, and Advance returns after exactly one complete frame.
	ClockVirtual
)

func (m ClockMode) String() string {
	switch m {
	case ClockWall:
		return "wall"
	case ClockVirtual:
		return "virtual"
	default:
		return "unknown"
	}
}

// Clock is the runtime's time service (QP-5). Runtime-thread owned: reads
// from other threads MUST go through RuntimeServices accessors (Runtime.Now
// is only called on the runtime thread; drive handlers only ever run there).
//
// The clock is the engine's single wall-clock read point: nothing else under
// runtime/ (or the demo) may call time.Now for engine semantics — Debt-4.
// The wall source is injected as a function value so the grep gate stays an
// honest "no consumer reads wall time directly" check.
type Clock struct {
	mode  ClockMode
	vNow  time.Time
	lastF time.Time
	// engineBase/wallBase map monotonic wall elapsed time onto the logical
	// engine timeline. Switching modes reanchors this mapping at the last
	// engine instant; parked physical time is never applied to app timers.
	engineBase time.Time
	wallBase   time.Time
	// wall is the single wall-clock seam (Debt-4); a function value both for
	// the grep gate above and for clock unit tests.
	wall func() time.Time
	// gestureActive is installed from the input phase: true while a platform
	// gesture (drag / touch sequence) is mid-flight. Mode switches refuse
	// while it returns true (QP-5).
	gestureActive func() bool
}

// ClockConfig selects the initial engine time domain. EpochNS is meaningful
// for Virtual startup and is an exact signed Unix-nanosecond instant.
type ClockConfig struct {
	InitialMode ClockMode
	EpochNS     int64
}

// EngineClock is the readonly clock view supplied to root builders.
type EngineClock interface {
	Now() time.Time
	Mode() ClockMode
}

// PreparedClock is created before application/root construction and is then
// installed unchanged into Runtime. Its view grants no clock mutation API.
type PreparedClock struct {
	clock     *Clock
	bootstrap bool
}

func (p *PreparedClock) View() EngineClock {
	if p == nil {
		return nil
	}
	return clockView{clock: p.clock}
}

type clockView struct{ clock *Clock }

func (v clockView) Now() time.Time  { return v.clock.Now() }
func (v clockView) Mode() ClockMode { return v.clock.Mode() }

// PrepareClock constructs the single engine clock before timed app consumers
// and root builders initialize state.
func PrepareClock(config ClockConfig) (*PreparedClock, error) {
	c := NewClock(wallNow)
	switch config.InitialMode {
	case ClockWall:
		if config.EpochNS != 0 {
			return nil, errors.New("runtime: EpochNS is only valid for virtual startup")
		}
	case ClockVirtual:
		c.mode = ClockVirtual
		c.vNow = time.Unix(0, config.EpochNS).UTC()
		c.lastF = time.Time{}
		return &PreparedClock{clock: c, bootstrap: true}, nil
	default:
		return nil, errors.New("runtime: unknown initial clock mode")
	}
	return &PreparedClock{clock: c}, nil
}

// NewClock returns a Wall-mode clock anchored to the given wall source.
func NewClock(wall func() time.Time) *Clock {
	if wall == nil {
		wall = time.Now
	}
	anchor := wall()
	return &Clock{mode: ClockWall, wall: wall, engineBase: anchor, wallBase: anchor}
}

// Mode reports the current mode.
func (c *Clock) Mode() ClockMode { return c.mode }

// Now returns the current engine time: wall reads in Wall mode, the frozen
// virtual instant in Virtual mode.
func (c *Clock) Now() time.Time {
	switch c.mode {
	case ClockVirtual:
		return c.vNow
	default:
		return c.engineBase.Add(c.wall().Sub(c.wallBase))
	}
}

// WallNow exposes the injected monotonic wall source for pacing, deadlines,
// and leases. It is not affected by virtual-time mode.
func (c *Clock) WallNow() time.Time { return c.wall() }

// SetGestureGuard installs the mid-gesture probe (from the input phase).
func (c *Clock) SetGestureGuard(fn func() bool) { c.gestureActive = fn }

// GestureActive reports whether a platform gesture is mid-flight.
func (c *Clock) GestureActive() bool {
	return c.gestureActive != nil && c.gestureActive()
}

// SetVirtual switches to Virtual mode, anchoring the virtual clock at the
// current wall instant. It refuses while a platform gesture is mid-flight
// (QP-5: mode switching must not sever an in-progress gesture).
func (c *Clock) SetVirtual() error {
	if c.mode == ClockVirtual {
		return nil
	}
	if c.GestureActive() {
		return ErrClockGestureActive
	}
	c.vNow = c.Now()
	c.lastF = time.Time{}
	c.mode = ClockVirtual
	return nil
}

// SetWall returns to Wall mode. Always allowed.
func (c *Clock) SetWall() {
	if c.mode == ClockWall {
		return
	}
	c.engineBase = c.vNow
	c.wallBase = c.wall()
	c.mode = ClockWall
	c.lastF = time.Time{}
}

// Advance moves virtual time forward by d and returns the instant the coming
// frame MUST observe. Wall mode ignores the request and returns the wall
// clock (advancing wall time is not a thing; callers gate on mode).
func (c *Clock) Advance(d time.Duration) time.Time {
	if c.mode != ClockVirtual {
		return c.Now()
	}
	c.vNow = c.vNow.Add(d)
	return c.vNow
}

// CanAdvance reports whether d can be added without overflowing the signed
// Unix-nanosecond wire domain used by LDP timestamps. It does not mutate the
// clock, so admission can reject invalid requests before claiming their IDs.
func (c *Clock) CanAdvance(d time.Duration) bool {
	if c == nil || c.mode != ClockVirtual || d <= 0 {
		return false
	}
	now := c.vNow.UnixNano()
	return now <= int64(^uint64(0)>>1)-int64(d)
}

// FrameDelta records the frame instant and returns the delta since the
// previous frame. First frame after a mode switch yields zero.
func (c *Clock) FrameDelta(now time.Time) time.Duration {
	if c.lastF.IsZero() {
		c.lastF = now
		return 0
	}
	dt := now.Sub(c.lastF)
	c.lastF = now
	return dt
}

// SeedLastFrame pins the previous-frame instant (test seam mirroring
// Runtime.SetFrameClock). The next FrameDelta is measured from it.
func (c *Clock) SeedLastFrame(now time.Time) { c.lastF = now }

// ErrClockGestureActive is returned by SetVirtual when a platform gesture is
// mid-flight (QP-5).
var ErrClockGestureActive = errClockGestureActive{}

type errClockGestureActive struct{}

func (errClockGestureActive) Error() string {
	return "clock: cannot enter virtual mode while a platform gesture is active"
}
