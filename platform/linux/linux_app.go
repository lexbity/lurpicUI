package linux

import (
	"errors"
	"sync"
	"time"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/platform/internal/common"
)

const (
	testEventKeyPress = iota + 1
	testEventKeyRelease
	testEventButtonPress
	testEventButtonRelease
	testEventMotion
	testEventEnter
	testEventLeave
	testEventFocusIn
	testEventFocusOut
	testEventExpose
	testEventConfigure
	testEventClientMessage
)

type testEvent struct {
	kind   int
	detail uint8
	eventX int16
	eventY int16
	state  uint16
	window uint32
	width  uint16
	height uint16
	data32 uint32
}

type app struct {
	mu             sync.Mutex
	windows        map[uint32]*window
	events         *eventQueue
	clipboard      *clipboard
	clipboardOwner uint32
	atomWMDelete   uint32
	atomUTF8String uint32
	atomClipboard  uint32
}

type window struct {
	app     *app
	surface *shmSurface
	id      uint32
	width   int
	height  int
	closed  bool
}

type shmSurface struct {
	buf    []byte
	stride int
	width  int
	height int
	locked bool
	mu     sync.Mutex
}

type eventQueue struct {
	mu      sync.Mutex
	pending []platform.Event
	app     *app
}

type clipboard struct {
	app  *app
	text string
	own  bool
}

var _ platform.App = (*app)(nil)
var _ platform.WindowCapable = (*app)(nil)
var _ platform.ClipboardCapable = (*app)(nil)

func (a *app) NewWindow(opts platform.WindowOptions) (platform.Window, error) {
	if opts.Width <= 0 || opts.Height <= 0 {
		return nil, errors.New("invalid size")
	}
	if a.windows == nil {
		a.windows = make(map[uint32]*window)
	}
	win := &window{
		app:     a,
		surface: &shmSurface{buf: make([]byte, opts.Width*opts.Height*4), stride: opts.Width * 4, width: opts.Width, height: opts.Height},
		id:      uint32(len(a.windows) + 1),
		width:   opts.Width,
		height:  opts.Height,
	}
	a.windows[win.id] = win
	return win, nil
}

func (a *app) Events() platform.EventQueue {
	if a == nil {
		return nil
	}
	return a.events
}

func (a *app) Clipboard() platform.Clipboard {
	if a == nil {
		return nil
	}
	return a.clipboard
}

func (a *app) Destroy() {
	if a == nil {
		return
	}
	a.mu.Lock()
	for _, w := range a.windows {
		if w != nil {
			w.destroy()
		}
	}
	a.windows = nil
	a.mu.Unlock()
}

func (w *window) Surface() platform.Surface      { return w.surface }
func (w *window) SetTitle(title string)          {}
func (w *window) Size() (width, height int)      { return w.width, w.height }
func (w *window) ContentScale() float32          { return 1 }
func (w *window) SetIMECursorRect(rect gfx.Rect) {}
func (w *window) Show()                          {}
func (w *window) Hide()                          {}
func (w *window) Close()                         { w.destroy() }
func (w *window) Destroy()                       { w.destroy() }

func (w *window) destroy() {
	if w == nil || w.closed {
		return
	}
	w.closed = true
	if w.app != nil {
		w.app.mu.Lock()
		delete(w.app.windows, w.id)
		w.app.mu.Unlock()
	}
}

func (s *shmSurface) Buffer() []byte            { return s.buf }
func (s *shmSurface) Stride() int               { return s.stride }
func (s *shmSurface) Size() (width, height int) { return s.width, s.height }
func (s *shmSurface) Scale() float32            { return 1 }
func (s *shmSurface) Lock() error {
	s.mu.Lock()
	s.locked = true
	return nil
}
func (s *shmSurface) Unlock([]gfx.Rect) error {
	s.locked = false
	s.mu.Unlock()
	return nil
}
func (s *shmSurface) Resize(width, height int) {
	if width <= 0 || height <= 0 {
		width = 1
		height = 1
	}
	s.width = width
	s.height = height
	s.stride = width * 4
	s.buf = make([]byte, width*height*4)
}
func (s *shmSurface) Destroy() {}

func (q *eventQueue) Push(e platform.Event) {
	if q == nil {
		return
	}
	q.mu.Lock()
	q.pending = append(q.pending, e)
	q.mu.Unlock()
}

func (q *eventQueue) Poll() []platform.Event {
	if q == nil {
		return nil
	}
	q.mu.Lock()
	if len(q.pending) == 0 {
		q.mu.Unlock()
		return nil
	}
	out := append([]platform.Event(nil), q.pending...)
	q.pending = q.pending[:0]
	q.mu.Unlock()
	return out
}
func (q *eventQueue) Wait(timeout time.Duration) []platform.Event {
	if q == nil {
		return nil
	}
	_ = timeout
	return q.Poll()
}

func (c *clipboard) ReadText() (string, error) {
	if c == nil || c.app == nil {
		return "", errors.New("clipboard unavailable")
	}
	if c.own {
		return c.text, nil
	}
	if c.app.clipboardOwner == 0 {
		return "", errors.New("clipboard unavailable")
	}
	return "", errors.New("clipboard unavailable")
}

func (c *clipboard) WriteText(text string) error {
	if c == nil || c.app == nil {
		return errors.New("clipboard unavailable")
	}
	if c.app.clipboardOwner == 0 {
		return errors.New("clipboard unavailable")
	}
	c.text = text
	c.own = true
	return nil
}

func (a *app) DestroyClipboard() {
	if a != nil && a.clipboard != nil {
		a.clipboard.own = false
	}
}

func (a *app) setClipboard(text string) {
	if a == nil {
		return
	}
	if a.clipboard == nil {
		return
	}
	a.clipboard.text = text
	a.clipboard.own = true
}

func (a *app) lookupWindow(id uint32) *window {
	if a == nil {
		return nil
	}
	return a.windows[id]
}

func (a *app) translateEvent(ev *testEvent) []platform.Event {
	if ev == nil {
		return nil
	}
	switch ev.kind {
	case testEventKeyPress:
		return a.translateKeyEvent(ev, true)
	case testEventKeyRelease:
		return a.translateKeyEvent(ev, false)
	case testEventButtonPress:
		return a.translatePointerButton(ev, true)
	case testEventButtonRelease:
		return a.translatePointerButton(ev, false)
	case testEventMotion:
		return a.translateMotion(ev)
	case testEventEnter:
		return a.translateEnterLeave(ev, true)
	case testEventLeave:
		return a.translateEnterLeave(ev, false)
	case testEventFocusIn:
		return a.translateFocus(ev, true)
	case testEventFocusOut:
		return a.translateFocus(ev, false)
	case testEventExpose:
		return a.translateExpose(ev)
	case testEventConfigure:
		return a.translateConfigure(ev)
	case testEventClientMessage:
		return a.translateClientMessage(ev)
	default:
		return nil
	}
}

func (a *app) translateKeyEvent(ev *testEvent, press bool) []platform.Event {
	key := common.KeyFromKeysym(uint32(ev.detail))
	mod := common.ModifiersFromState(ev.state)
	kind := platform.KeyRelease
	if press {
		kind = platform.KeyPress
	}
	return []platform.Event{
		platform.EventKey{Kind: kind, Key: key, Modifiers: mod},
	}
}

func (a *app) translatePointerButton(ev *testEvent, press bool) []platform.Event {
	button := platform.PointerNone
	switch ev.detail {
	case 1:
		button = platform.PointerLeft
	case 2:
		button = platform.PointerMiddle
	case 3:
		button = platform.PointerRight
	}
	kind := platform.PointerRelease
	if press {
		kind = platform.PointerPress
	}
	return []platform.Event{
		platform.EventPointer{
			Kind:      kind,
			Button:    button,
			Position:  gfx.Point{X: float32(ev.eventX), Y: float32(ev.eventY)},
			Modifiers: common.ModifiersFromState(ev.state),
		},
	}
}

func (a *app) translateMotion(ev *testEvent) []platform.Event {
	return []platform.Event{
		platform.EventPointer{
			Kind:      platform.PointerMove,
			Position:  gfx.Point{X: float32(ev.eventX), Y: float32(ev.eventY)},
			Modifiers: common.ModifiersFromState(ev.state),
		},
	}
}

func (a *app) translateEnterLeave(ev *testEvent, enter bool) []platform.Event {
	kind := platform.PointerLeave
	if enter {
		kind = platform.PointerEnter
	}
	return []platform.Event{
		platform.EventPointer{
			Kind:      kind,
			Position:  gfx.Point{X: float32(ev.eventX), Y: float32(ev.eventY)},
			Modifiers: common.ModifiersFromState(ev.state),
		},
	}
}

func (a *app) translateFocus(ev *testEvent, focused bool) []platform.Event {
	win := a.lookupWindow(ev.window)
	if win == nil {
		return nil
	}
	return []platform.Event{
		platform.EventWindowFocus{Window: win, Focused: focused},
	}
}

func (a *app) translateExpose(ev *testEvent) []platform.Event {
	win := a.lookupWindow(ev.window)
	if win == nil {
		return nil
	}
	return []platform.Event{
		platform.EventWindowResize{Window: win, Width: win.width, Height: win.height},
	}
}

func (a *app) translateConfigure(ev *testEvent) []platform.Event {
	win := a.lookupWindow(ev.window)
	if win == nil {
		return nil
	}
	win.width = int(ev.width)
	win.height = int(ev.height)
	if win.surface != nil {
		win.surface.Resize(int(ev.width), int(ev.height))
	}
	return []platform.Event{
		platform.EventWindowResize{Window: win, Width: int(ev.width), Height: int(ev.height)},
	}
}

func (a *app) translateClientMessage(ev *testEvent) []platform.Event {
	win := a.lookupWindow(ev.window)
	if win == nil {
		return nil
	}
	if ev.data32 != a.atomWMDelete {
		return nil
	}
	return []platform.Event{
		platform.EventWindowClose{Window: win},
	}
}
