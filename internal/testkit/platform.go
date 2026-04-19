package testkit

import (
	"sync"
	"time"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
)

// NullClipboard is an in-memory clipboard implementation.
type NullClipboard struct {
	mu   sync.Mutex
	text string
}

func (c *NullClipboard) ReadText() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.text, nil
}

func (c *NullClipboard) WriteText(text string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.text = text
	return nil
}

// nullEventQueue is a FIFO queue of synthetic events.
type nullEventQueue struct {
	mu     sync.Mutex
	events []platform.Event
}

func (q *nullEventQueue) Push(e platform.Event) {
	q.mu.Lock()
	q.events = append(q.events, e)
	q.mu.Unlock()
}

func (q *nullEventQueue) Poll() []platform.Event {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.events) == 0 {
		return nil
	}
	out := append([]platform.Event(nil), q.events...)
	q.events = q.events[:0]
	return out
}

func (q *nullEventQueue) Wait(timeout time.Duration) []platform.Event {
	_ = timeout
	return q.Poll()
}

// NullWindow is an in-memory platform.Window implementation.
type NullWindow struct {
	mu      sync.Mutex
	surface *MemorySurface
	title   string
	closed  bool
	shown   bool
}

func (w *NullWindow) Surface() platform.Surface {
	return w.surface
}

func (w *NullWindow) SetTitle(title string) {
	w.mu.Lock()
	w.title = title
	w.mu.Unlock()
}

func (w *NullWindow) Size() (width, height int) {
	return w.surface.Size()
}

func (w *NullWindow) ContentScale() float32 {
	return 1
}

func (w *NullWindow) SetIMECursorRect(rect gfx.Rect) {}

func (w *NullWindow) Resize(width, height int) {
	if w == nil || w.surface == nil {
		return
	}
	w.surface.Resize(width, height)
}

func (w *NullWindow) Show() {
	w.mu.Lock()
	w.shown = true
	w.mu.Unlock()
}

func (w *NullWindow) Hide() {
	w.mu.Lock()
	w.shown = false
	w.mu.Unlock()
}

func (w *NullWindow) Close() {
	w.mu.Lock()
	w.closed = true
	w.mu.Unlock()
}

func (w *NullWindow) Destroy() {
	w.Close()
}

// NullApp is a headless platform.App.
type NullApp struct {
	mu        sync.Mutex
	windows   []*NullWindow
	queue     nullEventQueue
	clipboard NullClipboard
	width     int
	height    int
	destroyed bool
}

// NewNullApp constructs a headless app with a default window size.
func NewNullApp(width, height int) *NullApp {
	return &NullApp{width: width, height: height}
}

func (a *NullApp) NewWindow(opts platform.WindowOptions) (platform.Window, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	w := opts.Width
	h := opts.Height
	if w <= 0 {
		w = a.width
	}
	if h <= 0 {
		h = a.height
	}
	window := &NullWindow{surface: NewMemorySurface(w, h)}
	if opts.Title != "" {
		window.title = opts.Title
	}
	a.windows = append(a.windows, window)
	return window, nil
}

func (a *NullApp) Events() platform.EventQueue { return &a.queue }

func (a *NullApp) Clipboard() platform.Clipboard { return &a.clipboard }

func (a *NullApp) Destroy() {
	a.mu.Lock()
	for _, w := range a.windows {
		if w != nil {
			w.Destroy()
		}
	}
	a.windows = nil
	a.destroyed = true
	a.mu.Unlock()
}

// InjectEvent appends an event to be returned by the next Poll call.
func (a *NullApp) InjectEvent(e platform.Event) {
	a.queue.Push(e)
}

var _ platform.App = (*NullApp)(nil)
var _ platform.Window = (*NullWindow)(nil)
var _ platform.Clipboard = (*NullClipboard)(nil)
var _ platform.EventQueue = (*nullEventQueue)(nil)

var _ = gfx.Point{}
