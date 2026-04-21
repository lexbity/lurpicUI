package platform

import (
	"time"

	"codeburg.org/lexbit/lurpicui/gfx"
)

type Surface interface {
	Buffer() []byte
	Stride() int
	Size() (width, height int)
	Lock() error
	Unlock(dirtyRects []gfx.Rect) error
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

type EventQueue interface {
	Poll() []Event
	Wait(timeout time.Duration) []Event
}

type Clipboard interface {
	ReadText() (string, error)
	WriteText(text string) error
}

type App interface {
	NewWindow(opts WindowOptions) (Window, error)
	Events() EventQueue
	Clipboard() Clipboard
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

func (EventWindowClose) isEvent()  {}
func (EventWindowResize) isEvent() {}
func (EventWindowFocus) isEvent()  {}
func (EventPointer) isEvent()      {}
func (EventScroll) isEvent()       {}
func (EventKey) isEvent()          {}
func (EventText) isEvent()         {}
func (EventIMECompose) isEvent()   {}
func (EventIMECommit) isEvent()    {}
