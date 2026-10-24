package runtime

import (
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
	// wall is the single wall-clock seam (Debt-4); a function value both for
	// the grep gate above and for clock unit tests.
	wall func() time.Time
	// gestureActive is installed from the input phase: true while a platform
	// gesture (drag / touch sequence) is mid-flight. Mode switches refuse
	// while it returns true (QP-5).
	gestureActive func() bool
}

// NewClock returns a Wall-mode clock anchored to the given wall source.
func NewClock(wall func() time.Time) *Clock {
	return &Clock{mode: ClockWall, wall: wall}
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
		return c.wall()
	}
}

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
	c.vNow = c.wall()
	c.lastF = time.Time{}
	c.mode = ClockVirtual
	return nil
}

// SetWall returns to Wall mode. Always allowed.
func (c *Clock) SetWall() {
	if c.mode == ClockWall {
		return
	}
	c.mode = ClockWall
	c.lastF = time.Time{}
}

// Advance moves virtual time forward by d and returns the instant the coming
// frame MUST observe. Wall mode ignores the request and returns the wall
// clock (advancing wall time is not a thing; callers gate on mode).
func (c *Clock) Advance(d time.Duration) time.Time {
	if c.mode != ClockVirtual {
		return c.wall()
	}
	c.vNow = c.vNow.Add(d)
	return c.vNow
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
