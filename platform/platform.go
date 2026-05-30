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

type EventPointer struct {
	Kind      PointerEventKind
	Position  gfx.Point
	Button    PointerButton
	Modifiers ModifierKeys
}

type EventScroll struct {
	Position  gfx.Point
	DeltaX    float32
	DeltaY    float32
	Precise   bool
	Modifiers ModifierKeys
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
	Kind      KeyEventKind
	Key       Key
	Modifiers ModifierKeys
}

type EventText struct {
	Text string
}

type EventIMECompose struct {
	Text      string
	CursorPos int
}

type EventIMECommit struct {
	Text string
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

// TouchEvent represents a touch contact event on Android.
// Multiple TouchEvents with the same SequenceID belong to one finger's gesture.
type TouchEvent struct {
	SequenceID uint64     // identifies this contact across down/move/up
	Phase      TouchPhase // Down, Move, Up, Cancel
	X, Y       float32    // surface-relative position
	Pressure   float32    // 0.0 to 1.0
}

// TouchPhase represents the phase of a touch event
type TouchPhase int

const (
	TouchDown TouchPhase = iota
	TouchMove
	TouchUp
	TouchCancel // OS canceled the gesture (e.g., system gesture took over)
)

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
