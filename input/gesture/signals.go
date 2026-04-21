package gesture

import (
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/input"
	"codeburg.org/lexbit/lurpicui/platform"
)

// TapSignal is emitted when a tap gesture completes.
type TapSignal struct {
	Position  gfx.Point
	TapCount  int
	Modifiers input.Modifiers
}

// LongPressSignal reports long-press lifecycle changes.
type LongPressSignal struct {
	State    RecognizerState
	Position gfx.Point
}

// PanSignal reports pan lifecycle and movement.
type PanSignal struct {
	State      RecognizerState
	Position   gfx.Point
	Delta      gfx.Point
	TotalDelta gfx.Point
	Velocity   gfx.Point
}

// SwipeSignal reports a completed swipe direction.
type SwipeSignal struct {
	Direction SwipeDirection
	Velocity  float32
}

// SwipeDirection identifies a swipe axis and direction.
type SwipeDirection uint8

const (
	SwipeLeft SwipeDirection = iota
	SwipeRight
	SwipeUp
	SwipeDown
)

// SwipeDirectionMask filters the directions a swipe recognizer accepts.
type SwipeDirectionMask uint8

const (
	SwipeMaskLeft  SwipeDirectionMask = 1 << SwipeLeft
	SwipeMaskRight SwipeDirectionMask = 1 << SwipeRight
	SwipeMaskUp    SwipeDirectionMask = 1 << SwipeUp
	SwipeMaskDown  SwipeDirectionMask = 1 << SwipeDown
	SwipeMaskAny   SwipeDirectionMask = 0xFF
)

// PinchSignal reports pinch scale changes.
type PinchSignal struct {
	State    RecognizerState
	Scale    float32
	Delta    float32
	Center   gfx.Point
	Velocity float32
}

// RotationSignal reports two-finger rotation changes.
type RotationSignal struct {
	State    RecognizerState
	Rotation float32
	Delta    float32
	Center   gfx.Point
	Velocity float32
}

type scrollGestureHandler interface {
	ScrollGesture(platform.EventScroll, InputEvent)
}

type recognizerDelegateHolder struct {
	Delegate RecognizerDelegate
}

// PlatformAdaptation selects which desktop event mappings are enabled.
type PlatformAdaptation uint8

const (
	PlatformAdaptationAuto PlatformAdaptation = iota
	PlatformAdaptationDesktop
)

// SignalEmitter may be implemented by recognizers that want to queue signals.
type SignalEmitter interface {
	QueueSignals(SignalQueue, []Touch, InputEvent)
}

// SignalQueue receives deferred signal delivery callbacks.
type SignalQueue interface {
	Enqueue(func())
}
