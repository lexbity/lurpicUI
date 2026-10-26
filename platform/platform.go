package platform

import (
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/render"
)

// Surface is backend-neutral; software-specific pixel access lives on render.SoftwareSurface.
type Surface interface {
	render.Surface
	Scale() float32
}

type Window interface {
	Surface() Surface
	SetTitle(title string)
	Size() (width, height int)
	ContentScale() float32
	SetIMECursorRect(rect gfx.Rect)
	Show()
	Hide()
	Close()
	Destroy()
}

type Clipboard interface {
	ReadText() (string, error)
	WriteText(text string) error
}

type App interface {
	Events() EventQueue
	SetFrameDemand(bool)
	Destroy()
}

type WindowOptions struct {
	Title     string
	Width     int
	Height    int
	Resizable bool
	MinSize   gfx.Size
	MaxSize   gfx.Size
}

type Event interface {
	isEvent()
}

type EventWindowClose struct {
	Window Window
}

type EventWindowResize struct {
	Window Window
	Width  int
	Height int
}

type EventWindowFocus struct {
	Window  Window
	Focused bool
}

type PointerButton uint8

const (
	PointerNone PointerButton = iota
	PointerLeft
	PointerMiddle
	PointerRight
)

type PointerEventKind uint8

const (
	PointerMove PointerEventKind = iota
	PointerPress
	PointerRelease
	PointerEnter
	PointerLeave
	PointerCancel
)

// EventSource names who injected an input event (LDP QP-10): the platform
// bridge (a human device) or the drive (LDP input synthesis). Events flow
// through hit-testing, capture, and gesture recognition identically —
// provenance is observable, never a shortcut. Marks that are
// security-sensitive may refuse drive-sourced input via
// facet.EventIsSynthetic.
type EventSource uint8

const (
	// SourcePlatform is the human input path. Every platform-bridge
	// construction site sets it explicitly; the zero value is not a valid
	// source.
	SourcePlatform EventSource = iota + 1
	// SourceDrive marks LDP-synthesized input.
	SourceDrive
	// SourceRuntimeCancel marks framework cancellation routed to recognizers.
	SourceRuntimeCancel
)

func (s EventSource) String() string {
	switch s {
	case SourcePlatform:
		return "platform"
	case SourceDrive:
		return "drive"
	case SourceRuntimeCancel:
		return "runtime_cancel"
	default:
		return "EventSource(unknown)"
	}
}

type EventPointer struct {
	Kind        PointerEventKind
	Position    gfx.Point
	Button      PointerButton
	Modifiers   ModifierKeys
	Source      EventSource
	Attribution InputAttribution
}

type EventScroll struct {
	Position    gfx.Point
	DeltaX      float32
	DeltaY      float32
	Precise     bool
	Modifiers   ModifierKeys
	Source      EventSource
	Attribution InputAttribution
}

type KeyEventKind uint8

const (
	KeyPress KeyEventKind = iota
	KeyRelease
	KeyRepeat
)

type Key uint16

const (
	KeyUnknown Key = iota
	KeyA
	KeyB
	KeyC
	KeyD
	KeyE
	KeyF
	KeyG
	KeyH
	KeyI
	KeyJ
	KeyK
	KeyL
	KeyM
	KeyN
	KeyO
	KeyP
	KeyQ
	KeyR
	KeyS
	KeyT
	KeyU
	KeyV
	KeyW
	KeyX
	KeyY
	KeyZ
	KeyLeft
	KeyRight
	KeyUp
	KeyDown
	KeyHome
	KeyEnd
	KeyPageUp
	KeyPageDown
	KeyEscape
	KeyEnter
	KeySpace
	KeyTab
	KeyBackspace
)

type EventKey struct {
	Kind        KeyEventKind
	Key         Key
	Modifiers   ModifierKeys
	Source      EventSource
	Attribution InputAttribution
}

type EventText struct {
	Text        string
	Source      EventSource
	Attribution InputAttribution
}

type EventIMECompose struct {
	Text        string
	CursorPos   int
	Source      EventSource
	Attribution InputAttribution
}

type EventIMECommit struct {
	Text        string
	Source      EventSource
	Attribution InputAttribution
}

// InputAttribution identifies the operation that produced a synthesized
// event. Platform events leave it empty; it contains no credential material.
type InputAttribution struct {
	Session    string
	Generation uint64
	Operation  string
}

type ModifierKeys uint8

const (
	ModShift ModifierKeys = 1 << iota
	ModControl
	ModAlt
	ModSuper
)

// LifecycleEvent represents Android lifecycle state changes
type LifecycleEvent struct {
	Kind LifecycleKind
}

// LifecycleKind represents the specific lifecycle state
type LifecycleKind int

const (
	LifecycleStart LifecycleKind = iota
	LifecycleResume
	LifecyclePause
	LifecycleStop
	LifecycleDestroy
	LifecycleLowMemory
)

// WindowEvent represents window-related events on Android
type WindowEvent struct {
	Kind   WindowEventKind
	Window uintptr
	Width  int
	Height int
}

// WindowEventKind represents the specific window event type
type WindowEventKind int

const (
	WindowCreated WindowEventKind = iota
	WindowResized
	WindowDestroyed
	WindowFocusGained
	WindowFocusLost
)

// VsyncEvent is emitted by the Android Choreographer on each vsync.
// frameTimeNanos is the expected presentation time in nanoseconds.
type VsyncEvent struct {
	FrameTimeNanos int64
}

// BackEvent is emitted when the system back gesture/button is invoked.
// On Android 33+ this uses the predictive back animation (OnBackInvokedCallback).
type BackEvent struct{}

// AudioFocusEvent is emitted when the Android audio focus changes.
// The runtime delivers this to the audio subsystem to pause/resume/duck playback.
type AudioFocusEvent struct {
	Change AudioFocusChange
}

// TrimMemoryEvent is delivered when the platform signals memory pressure
// with a severity level (e.g., Android's onTrimMemory).
type TrimMemoryEvent struct {
	Level int
}

// ConfigurationChangedEvent is emitted when the Android device configuration
// changes (orientation, density, fontScale, dark mode, locale). The runtime
// uses it to trigger re-layout and theme updates without losing view state.
type ConfigurationChangedEvent struct {
	Orientation    int     // ACONFIGURATION_ORIENTATION_PORT (1) or LAND (2)
	ScreenWidthDp  int     // screen width in density-independent pixels
	ScreenHeightDp int     // screen height in density-independent pixels
	Density        int     // ACONFIGURATION_DENSITY_* constant
	UiModeNight    bool    // true if UI mode is night (dark theme)
	FontScale      float32 // user font scale setting (1.0 = default)
	Language       string  // ISO 639-1 two-letter language code
	Country        string  // ISO 3166-1 two-letter country code
}

// TouchEvent represents a touch contact event on Android.
// Multiple TouchEvents with the same SequenceID belong to one finger's gesture.
type TouchEvent struct {
	SequenceID  uint64     // identifies this contact across down/move/up
	Phase       TouchPhase // Down, Move, Up, Cancel
	X, Y        float32    // surface-relative position
	Pressure    float32    // 0.0 to 1.0
	Source      EventSource
	Attribution InputAttribution
}

// IsInputEvent classifies only human-actuatable input. Lifecycle, vsync,
// and routine window bookkeeping do not preempt a controller.
func IsInputEvent(event Event) bool {
	switch event.(type) {
	case EventPointer, EventScroll, EventKey, EventText, EventIMECompose, EventIMECommit, TouchEvent:
		return true
	default:
		return false
	}
}

func EventSourceOf(event Event) EventSource {
	switch e := event.(type) {
	case EventPointer:
		return e.Source
	case EventScroll:
		return e.Source
	case EventKey:
		return e.Source
	case EventText:
		return e.Source
	case EventIMECompose:
		return e.Source
	case EventIMECommit:
		return e.Source
	case TouchEvent:
		return e.Source
	default:
		return SourceRuntimeCancel
	}
}

func InputAttributionOf(event Event) InputAttribution {
	switch e := event.(type) {
	case EventPointer:
		return e.Attribution
	case EventScroll:
		return e.Attribution
	case EventKey:
		return e.Attribution
	case EventText:
		return e.Attribution
	case EventIMECompose:
		return e.Attribution
	case EventIMECommit:
		return e.Attribution
	case TouchEvent:
		return e.Attribution
	default:
		return InputAttribution{}
	}
}

// TouchPhase represents the phase of a touch event
type TouchPhase int

const (
	TouchDown TouchPhase = iota
	TouchMove
	TouchUp
	TouchCancel // OS canceled the gesture (e.g., system gesture took over)
)

func (BackEvent) isEvent() {}

func (VsyncEvent) isEvent() {}

func (AudioFocusEvent) isEvent() {}

func (ConfigurationChangedEvent) isEvent() {}

func (TrimMemoryEvent) isEvent() {}

func (EventWindowClose) isEvent()  {}
func (EventWindowResize) isEvent() {}
func (EventWindowFocus) isEvent()  {}
func (EventPointer) isEvent()      {}
func (EventScroll) isEvent()       {}
func (EventKey) isEvent()          {}
func (EventText) isEvent()         {}
func (EventIMECompose) isEvent()   {}
func (EventIMECommit) isEvent()    {}
func (LifecycleEvent) isEvent()    {}
func (WindowEvent) isEvent()       {}
func (TouchEvent) isEvent()        {}
