//go:build linux && cgo

package linux

/*
#cgo pkg-config: xcb xcb-shm xcb-icccm xcb-keysyms

#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <unistd.h>
#include <sys/ipc.h>
#include <sys/shm.h>
#include <poll.h>
#include <xcb/xcb.h>
#include <xcb/shm.h>
#include <xcb/xcb_icccm.h>
#include <xcb/xcb_keysyms.h>
#include <X11/keysym.h>

static int lurpic_shmget(size_t size) {
	return shmget(IPC_PRIVATE, size, IPC_CREAT | 0600);
}

static void* lurpic_shmat(int id) {
	return shmat(id, NULL, 0);
}

static int lurpic_shmdt(void* addr) {
	return shmdt(addr);
}

static int lurpic_shmrm(int id) {
	return shmctl(id, IPC_RMID, 0);
}

static int lurpic_shmat_failed(void* addr) {
	return addr == (void*)-1;
}

static void lurpic_xcb_create_window(
	xcb_connection_t *conn,
	uint8_t depth,
	xcb_window_t wid,
	xcb_window_t parent,
	int16_t x,
	int16_t y,
	uint16_t width,
	uint16_t height,
	uint16_t border_width,
	uint16_t _class,
	xcb_visualid_t visual,
	uint32_t value_mask,
	uint32_t *value_list
) {
	xcb_create_window(conn, depth, wid, parent, x, y, width, height, border_width, _class, visual, value_mask, value_list);
}

static void lurpic_xcb_shm_put_image(
	xcb_connection_t *conn,
	xcb_drawable_t drawable,
	xcb_gcontext_t gc,
	uint16_t total_width,
	uint16_t total_height,
	int16_t src_x,
	int16_t src_y,
	uint16_t src_width,
	uint16_t src_height,
	int16_t dst_x,
	int16_t dst_y,
	uint8_t depth,
	uint8_t format,
	uint8_t send_event,
	xcb_shm_seg_t shmseg,
	uint32_t offset
) {
	xcb_shm_put_image(conn, drawable, gc, total_width, total_height, src_x, src_y, src_width, src_height, dst_x, dst_y, depth, format, send_event, shmseg, offset);
}

static uint32_t lurpic_client_message_data32(const xcb_client_message_event_t *ev) {
	return ev->data.data32[0];
}

static uint8_t lurpic_event_type(const xcb_generic_event_t *ev) {
	return ev->response_type & 0x7f;
}

static uint32_t lurpic_selection_notify_property(const xcb_selection_notify_event_t *ev) {
	return ev->property;
}

static uint32_t lurpic_selection_notify_requestor(const xcb_selection_notify_event_t *ev) {
	return ev->requestor;
}

static uint32_t lurpic_selection_notify_target(const xcb_selection_notify_event_t *ev) {
	return ev->target;
}

static void lurpic_xcb_convert_selection(
	xcb_connection_t *conn,
	xcb_window_t requestor,
	xcb_atom_t selection,
	xcb_atom_t target,
	xcb_atom_t property,
	xcb_timestamp_t time
) {
	xcb_convert_selection(conn, requestor, selection, target, property, time);
}

static int lurpic_xcb_get_property_value_length(xcb_get_property_reply_t *reply) {
	return xcb_get_property_value_length(reply);
}

static char* lurpic_xcb_get_property_value(xcb_get_property_reply_t *reply) {
	return (char*)xcb_get_property_value(reply);
}
*/
import "C"

import (
	"errors"
	"math"
	"os"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
)

var errNoDisplay = errors.New("linux platform: no X display available")

func NewApp() (platform.App, error) {
	if os.Getenv("DISPLAY") == "" {
		return nil, errNoDisplay
	}

	conn := C.xcb_connect(nil, nil)
	if conn == nil || C.xcb_connection_has_error(conn) != 0 {
		if conn != nil {
			C.xcb_disconnect(conn)
		}
		return nil, errNoDisplay
	}

	setup := C.xcb_get_setup(conn)
	if setup == nil {
		C.xcb_disconnect(conn)
		return nil, errors.New("linux platform: xcb setup unavailable")
	}

	iter := C.xcb_setup_roots_iterator(setup)
	if iter.rem == 0 {
		C.xcb_disconnect(conn)
		return nil, errors.New("linux platform: no X11 screens available")
	}

	screen := iter.data
	app := &app{
		conn:      conn,
		screen:    screen,
		epollFD:   -1,
		windows:   make(map[uint32]*window),
		keysyms:   C.xcb_key_symbols_alloc(conn),
		clipboard: &clipboard{},
	}
	app.clipboard.app = app
	app.events = &eventQueue{app: app}

	epfd, err := syscall.EpollCreate1(syscall.EPOLL_CLOEXEC)
	if err != nil {
		app.Destroy()
		return nil, errors.New("linux platform: epoll create failed")
	}
	app.epollFD = epfd
	ev := &syscall.EpollEvent{Events: syscall.EPOLLIN, Fd: int32(C.xcb_get_file_descriptor(conn))}
	if err := syscall.EpollCtl(app.epollFD, syscall.EPOLL_CTL_ADD, int(ev.Fd), ev); err != nil {
		app.Destroy()
		return nil, errors.New("linux platform: epoll ctl failed")
	}

	if app.keysyms == nil {
		app.Destroy()
		return nil, errors.New("linux platform: key symbols unavailable")
	}

	if err := app.initAtoms(); err != nil {
		app.Destroy()
		return nil, err
	}

	app.clipboardOwner = C.xcb_generate_id(conn)
	C.lurpic_xcb_create_window(
		conn,
		C.uint8_t(1),
		app.clipboardOwner,
		screen.root,
		0,
		0,
		1,
		1,
		0,
		C.uint16_t(C.XCB_WINDOW_CLASS_INPUT_OUTPUT),
		screen.root_visual,
		0,
		nil,
	)

	return app, nil
}

type app struct {
	conn *C.xcb_connection_t

	screen         *C.xcb_screen_t
	keysyms        *C.xcb_key_symbols_t
	epollFD        int
	clipboardOwner C.xcb_window_t

	mu        sync.Mutex
	windows   map[uint32]*window
	events    *eventQueue
	clipboard *clipboard

	atomWMProtocols C.xcb_atom_t
	atomWMDelete    C.xcb_atom_t
	atomUTF8String  C.xcb_atom_t
	atomClipboard   C.xcb_atom_t
	atomTargets     C.xcb_atom_t
	atomPrimary     C.xcb_atom_t
	atomLurpicClip  C.xcb_atom_t
}

type window struct {
	app *app

	id       C.xcb_window_t
	gc       C.xcb_gcontext_t
	surface  *shmSurface
	visual   *C.xcb_visualtype_t
	colormap C.xcb_colormap_t
	width    int
	height   int
	title    string
	closed   bool
}

type shmSurface struct {
	mu     sync.Mutex
	cond   *sync.Cond
	conn   *C.xcb_connection_t
	win    C.xcb_window_t
	seg    C.xcb_shm_seg_t
	shmid  C.int
	data   unsafe.Pointer
	buf    []byte
	stride int
	width  int
	height int
	depth  int
	window *window
	locked bool
}

type eventQueue struct {
	app *app
}

type clipboard struct {
	app  *app
	text string
	own  bool
}

func (a *app) initAtoms() error {
	a.atomWMProtocols = internAtom(a.conn, "WM_PROTOCOLS")
	a.atomWMDelete = internAtom(a.conn, "WM_DELETE_WINDOW")
	a.atomUTF8String = internAtom(a.conn, "UTF8_STRING")
	a.atomClipboard = internAtom(a.conn, "CLIPBOARD")
	a.atomTargets = internAtom(a.conn, "TARGETS")
	a.atomPrimary = internAtom(a.conn, "PRIMARY")
	a.atomLurpicClip = internAtom(a.conn, "LURPICUI_CLIPBOARD")
	if a.atomWMProtocols == C.XCB_ATOM_NONE || a.atomWMDelete == C.XCB_ATOM_NONE || a.atomUTF8String == C.XCB_ATOM_NONE {
		return errors.New("linux platform: failed to intern required atoms")
	}
	return nil
}

func internAtom(conn *C.xcb_connection_t, name string) C.xcb_atom_t {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	cookie := C.xcb_intern_atom(conn, 0, C.uint16_t(len(name)), cname)
	reply := C.xcb_intern_atom_reply(conn, cookie, nil)
	if reply == nil {
		return C.XCB_ATOM_NONE
	}
	defer C.free(unsafe.Pointer(reply))
	return reply.atom
}

func findARGB32Visual(screen *C.xcb_screen_t) (*C.xcb_visualtype_t, int) {
	if screen == nil {
		return nil, 0
	}
	for depthIter := C.xcb_screen_allowed_depths_iterator(screen); depthIter.rem != 0; C.xcb_depth_next(&depthIter) {
		depth := (*C.xcb_depth_t)(depthIter.data)
		if depth == nil || depth.depth != 32 {
			continue
		}
		for visIter := C.xcb_depth_visuals_iterator(depth); visIter.rem != 0; C.xcb_visualtype_next(&visIter) {
			visual := (*C.xcb_visualtype_t)(visIter.data)
			if visual != nil && visual._class == C.XCB_VISUAL_CLASS_TRUE_COLOR {
				return visual, 32
			}
		}
	}
	return nil, 0
}

func (a *app) NewWindow(opts platform.WindowOptions) (platform.Window, error) {
	visual, depth := findARGB32Visual(a.screen)
	if visual == nil {
		return nil, errors.New("linux platform: no 32-bit ARGB visual available")
	}

	id := C.xcb_generate_id(a.conn)
	colormap := C.xcb_generate_id(a.conn)
	values := []C.uint32_t{
		C.uint32_t(C.XCB_EVENT_MASK_EXPOSURE |
			C.XCB_EVENT_MASK_STRUCTURE_NOTIFY |
			C.XCB_EVENT_MASK_KEY_PRESS |
			C.XCB_EVENT_MASK_KEY_RELEASE |
			C.XCB_EVENT_MASK_BUTTON_PRESS |
			C.XCB_EVENT_MASK_BUTTON_RELEASE |
			C.XCB_EVENT_MASK_POINTER_MOTION |
			C.XCB_EVENT_MASK_ENTER_WINDOW |
			C.XCB_EVENT_MASK_LEAVE_WINDOW |
			C.XCB_EVENT_MASK_FOCUS_CHANGE),
	}
	mask := C.uint32_t(C.XCB_CW_EVENT_MASK)

	if opts.Width <= 0 {
		opts.Width = 1
	}
	if opts.Height <= 0 {
		opts.Height = 1
	}

	C.xcb_create_colormap(a.conn, C.XCB_COLORMAP_ALLOC_NONE, colormap, a.screen.root, visual.visual_id)
	C.lurpic_xcb_create_window(
		a.conn,
		C.uint8_t(depth),
		id,
		a.screen.root,
		0,
		0,
		C.uint16_t(opts.Width),
		C.uint16_t(opts.Height),
		0,
		C.uint16_t(C.XCB_WINDOW_CLASS_INPUT_OUTPUT),
		visual.visual_id,
		mask,
		(*C.uint32_t)(unsafe.Pointer(&values[0])),
	)

	wmProtocols := []C.xcb_atom_t{a.atomWMDelete}
	C.xcb_change_property(
		a.conn,
		C.uint8_t(C.XCB_PROP_MODE_REPLACE),
		id,
		a.atomWMProtocols,
		C.XCB_ATOM_ATOM,
		32,
		1,
		unsafe.Pointer(&wmProtocols[0]),
	)

	surface, err := newSHMSurface(a.conn, id, opts.Width, opts.Height, int(depth))
	if err != nil {
		C.xcb_destroy_window(a.conn, id)
		C.xcb_free_colormap(a.conn, colormap)
		C.xcb_flush(a.conn)
		return nil, err
	}

	gc := C.xcb_generate_id(a.conn)
	C.xcb_create_gc(a.conn, gc, id, 0, nil)

	win := &window{
		app:      a,
		id:       id,
		gc:       gc,
		surface:  surface,
		visual:   visual,
		colormap: colormap,
		width:    opts.Width,
		height:   opts.Height,
	}
	surface.window = win
	a.mu.Lock()
	a.windows[uint32(id)] = win
	a.mu.Unlock()

	if opts.Title != "" {
		win.SetTitle(opts.Title)
	}
	C.xcb_flush(a.conn)
	return win, nil
}

func (a *app) Events() platform.EventQueue { return a.events }

func (a *app) Clipboard() platform.Clipboard { return a.clipboard }

func (a *app) Destroy() {
	if a == nil || a.conn == nil {
		return
	}
	a.mu.Lock()
	for _, w := range a.windows {
		w.destroy()
	}
	a.windows = nil
	a.mu.Unlock()
	if a.keysyms != nil {
		C.xcb_key_symbols_free(a.keysyms)
		a.keysyms = nil
	}
	if a.clipboardOwner != 0 {
		C.xcb_destroy_window(a.conn, a.clipboardOwner)
		a.clipboardOwner = 0
	}
	if a.epollFD >= 0 {
		_ = syscall.Close(a.epollFD)
		a.epollFD = -1
	}
	C.xcb_disconnect(a.conn)
	a.conn = nil
}

func (w *window) Surface() platform.Surface { return w.surface }

func (w *window) SetTitle(title string) {
	if w == nil || w.app == nil || w.closed {
		return
	}
	w.title = title
	cstr := C.CString(title)
	defer C.free(unsafe.Pointer(cstr))
	C.xcb_change_property(
		w.app.conn,
		C.uint8_t(C.XCB_PROP_MODE_REPLACE),
		w.id,
		C.XCB_ATOM_WM_NAME,
		w.app.atomUTF8String,
		8,
		C.uint32_t(len(title)),
		unsafe.Pointer(cstr),
	)
	C.xcb_flush(w.app.conn)
}

func (w *window) Size() (width, height int) { return w.width, w.height }

func (w *window) ContentScale() float32 { return 1 }

func (w *window) SetIMECursorRect(rect gfx.Rect) {}

func (w *window) Show() {
	if w == nil || w.app == nil || w.closed {
		return
	}
	C.xcb_map_window(w.app.conn, w.id)
	C.xcb_flush(w.app.conn)
}

func (w *window) Hide() {
	if w == nil || w.app == nil || w.closed {
		return
	}
	C.xcb_unmap_window(w.app.conn, w.id)
	C.xcb_flush(w.app.conn)
}

func (w *window) Close() {
	if w == nil || w.closed {
		return
	}
	w.destroy()
}

func (w *window) Destroy() { w.destroy() }

func (w *window) destroy() {
	if w == nil || w.closed {
		return
	}
	w.closed = true
	if w.surface != nil {
		w.surface.Destroy()
		w.surface = nil
	}
	if w.app != nil && w.app.conn != nil {
		if w.gc != 0 {
			C.xcb_free_gc(w.app.conn, w.gc)
		}
		if w.id != 0 {
			C.xcb_destroy_window(w.app.conn, w.id)
		}
		if w.colormap != 0 {
			C.xcb_free_colormap(w.app.conn, w.colormap)
		}
		C.xcb_flush(w.app.conn)
	}
	if w.app != nil {
		w.app.mu.Lock()
		delete(w.app.windows, uint32(w.id))
		w.app.mu.Unlock()
	}
}

func newSHMSurface(conn *C.xcb_connection_t, win C.xcb_window_t, width, height, depth int) (*shmSurface, error) {
	stride := align(width*4, 4)
	if stride <= 0 {
		stride = 4
	}
	size := stride * height
	if size <= 0 {
		size = stride
	}

	shmid := C.lurpic_shmget(C.size_t(size))
	if shmid < 0 {
		return nil, errors.New("linux platform: shmget failed")
	}

	data := C.lurpic_shmat(shmid)
	if C.lurpic_shmat_failed(data) != 0 || data == nil {
		C.lurpic_shmrm(shmid)
		return nil, errors.New("linux platform: shmat failed")
	}

	surface := &shmSurface{
		conn:   conn,
		win:    win,
		shmid:  shmid,
		data:   data,
		stride: stride,
		width:  width,
		height: height,
		depth:  depth,
		buf:    unsafe.Slice((*byte)(data), size),
	}
	surface.cond = sync.NewCond(&surface.mu)

	seg := C.xcb_generate_id(conn)
	surface.seg = C.xcb_shm_seg_t(seg)
	C.xcb_shm_attach(conn, surface.seg, C.uint32_t(shmid), 0)
	C.xcb_flush(conn)
	return surface, nil
}

func (s *shmSurface) Buffer() []byte { return s.buf }

func (s *shmSurface) Stride() int { return s.stride }

func (s *shmSurface) Size() (width, height int) { return s.width, s.height }

func (s *shmSurface) Lock() error {
	s.mu.Lock()
	for s.locked {
		s.cond.Wait()
	}
	s.locked = true
	return nil
}

func (s *shmSurface) Unlock(dirtyRects []gfx.Rect) error {
	if s == nil {
		return nil
	}
	if !s.locked {
		return nil
	}
	rects := dirtyRects
	if len(rects) == 0 {
		rects = []gfx.Rect{gfx.RectFromXYWH(0, 0, float32(s.width), float32(s.height))}
	}
	for _, r := range rects {
		if r.IsEmpty() {
			continue
		}
		x := int16(r.Min.X)
		y := int16(r.Min.Y)
		w := uint16(r.Width())
		h := uint16(r.Height())
		offset := uint32(int(r.Min.Y)*s.stride + int(r.Min.X)*4)
		C.lurpic_xcb_shm_put_image(
			s.conn,
			C.xcb_drawable_t(s.win),
			C.xcb_gcontext_t(0),
			C.uint16_t(w),
			C.uint16_t(h),
			C.int16_t(x),
			C.int16_t(y),
			C.uint16_t(w),
			C.uint16_t(h),
			0,
			0,
			C.uint8_t(32),
			C.XCB_IMAGE_FORMAT_Z_PIXMAP,
			C.uint8_t(0),
			s.seg,
			C.uint32_t(offset),
		)
	}
	C.xcb_flush(s.conn)
	s.locked = false
	s.mu.Unlock()
	s.cond.Broadcast()
	return nil
}

func (s *shmSurface) Destroy() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data != nil {
		C.lurpic_shmdt(s.data)
		s.data = nil
	}
	if s.shmid >= 0 {
		C.lurpic_shmrm(s.shmid)
		s.shmid = -1
	}
}

func (q *eventQueue) Poll() []platform.Event {
	if q == nil || q.app == nil || q.app.conn == nil {
		return nil
	}
	var events []platform.Event
	for {
		ev := C.xcb_poll_for_event(q.app.conn)
		if ev == nil {
			break
		}
		if translated := q.app.translateEvent(ev); translated != nil {
			events = append(events, translated...)
		}
		C.free(unsafe.Pointer(ev))
	}
	return events
}

func (q *eventQueue) Wait(timeout time.Duration) []platform.Event {
	if q == nil || q.app == nil || q.app.conn == nil {
		return nil
	}
	if pending := q.Poll(); len(pending) > 0 {
		return pending
	}
	if q.app.epollFD >= 0 {
		var events [1]syscall.EpollEvent
		ms := -1
		if timeout >= 0 {
			ms = int(timeout / time.Millisecond)
			if timeout > 0 && ms == 0 {
				ms = 1
			}
		}
		if n, err := syscall.EpollWait(q.app.epollFD, events[:], ms); err == nil && n > 0 {
			return q.Poll()
		}
	}
	return q.Poll()
}

func (c *clipboard) ReadText() (string, error) {
	if c == nil || c.app == nil {
		return "", errors.New("linux platform: clipboard unavailable")
	}
	c.app.mu.Lock()
	if c.own {
		c.app.mu.Unlock()
		return c.text, nil
	}
	conn := c.app.conn
	owner := c.app.clipboardOwner
	atomClipboard := c.app.atomClipboard
	atomUTF8String := c.app.atomUTF8String
	atomProperty := c.app.atomLurpicClip
	c.app.mu.Unlock()

	if owner == 0 {
		return "", errors.New("linux platform: clipboard owner unavailable")
	}

	C.lurpic_xcb_convert_selection(conn, owner, atomClipboard, atomUTF8String, atomProperty, C.XCB_CURRENT_TIME)
	C.xcb_flush(conn)

	for {
		ev := C.xcb_wait_for_event(conn)
		if ev == nil {
			return "", errors.New("linux platform: clipboard selection wait failed")
		}
		etype := C.lurpic_event_type(ev)
		if etype == C.XCB_SELECTION_NOTIFY {
			sn := (*C.xcb_selection_notify_event_t)(unsafe.Pointer(ev))
			if C.lurpic_selection_notify_requestor(sn) == C.xcb_window_t(owner) &&
				C.lurpic_selection_notify_target(sn) == atomUTF8String {
				prop := C.lurpic_selection_notify_property(sn)
				C.free(unsafe.Pointer(ev))
				if prop == C.XCB_ATOM_NONE {
					return "", errors.New("linux platform: clipboard selection unavailable")
				}
				reply := C.xcb_get_property_reply(
					conn,
					C.xcb_get_property(
						conn,
						0,
						owner,
						prop,
						atomUTF8String,
						0,
						1<<20,
					),
					nil,
				)
				if reply == nil {
					return "", errors.New("linux platform: clipboard property unavailable")
				}
				defer C.free(unsafe.Pointer(reply))
				length := int(C.lurpic_xcb_get_property_value_length(reply))
				value := C.lurpic_xcb_get_property_value(reply)
				if value == nil || length == 0 {
					return "", nil
				}
				return C.GoStringN(value, C.int(length)), nil
			}
		}
		if translated := c.app.translateEvent(ev); translated != nil {
			_ = translated
		}
		C.free(unsafe.Pointer(ev))
	}
}

func (c *clipboard) WriteText(text string) error {
	if c == nil || c.app == nil || c.app.conn == nil {
		return errors.New("linux platform: clipboard unavailable")
	}
	c.app.mu.Lock()
	c.text = text
	c.own = true
	c.app.mu.Unlock()

	str := C.CString(text)
	defer C.free(unsafe.Pointer(str))
	owner := c.app.clipboardOwner
	if owner == 0 {
		owner = C.xcb_window_t(c.app.screen.root)
	}
	C.xcb_set_selection_owner(c.app.conn, owner, c.app.atomClipboard, C.XCB_CURRENT_TIME)
	C.xcb_flush(c.app.conn)
	return nil
}

func (a *app) translateEvent(ev *C.xcb_generic_event_t) []platform.Event {
	if ev == nil {
		return nil
	}
	kind := ev.response_type & 0x7f
	switch kind {
	case C.XCB_KEY_PRESS, C.XCB_KEY_RELEASE:
		return a.translateKeyEvent(ev, kind == C.XCB_KEY_PRESS)
	case C.XCB_BUTTON_PRESS, C.XCB_BUTTON_RELEASE:
		return a.translatePointerButton(ev, kind == C.XCB_BUTTON_PRESS)
	case C.XCB_MOTION_NOTIFY:
		return a.translateMotion(ev)
	case C.XCB_ENTER_NOTIFY:
		return a.translateEnterLeave(ev, true)
	case C.XCB_LEAVE_NOTIFY:
		return a.translateEnterLeave(ev, false)
	case C.XCB_FOCUS_IN:
		return a.translateFocus(ev, true)
	case C.XCB_FOCUS_OUT:
		return a.translateFocus(ev, false)
	case C.XCB_CONFIGURE_NOTIFY:
		return a.translateConfigure(ev)
	case C.XCB_CLIENT_MESSAGE:
		return a.translateClientMessage(ev)
	case C.XCB_SELECTION_REQUEST:
		a.handleSelectionRequest(ev)
		return nil
	case C.XCB_SELECTION_NOTIFY:
		return nil
	default:
		return nil
	}
}

func (a *app) translateKeyEvent(ev *C.xcb_generic_event_t, press bool) []platform.Event {
	k := (*C.xcb_key_press_event_t)(unsafe.Pointer(ev))
	var keysym C.xcb_keysym_t
	if a.keysyms != nil {
		keysym = C.xcb_key_symbols_get_keysym(a.keysyms, C.xcb_keycode_t(k.detail), 0)
	}
	key := keyFromKeysym(uint32(keysym))
	mod := modifiersFromState(uint16(k.state))
	kind := platform.KeyRelease
	if press {
		kind = platform.KeyPress
	}
	out := []platform.Event{
		platform.EventKey{Kind: kind, Key: key, Modifiers: mod},
	}
	if press {
		if text, ok := textFromKeysym(uint32(keysym)); ok {
			out = append(out, platform.EventText{Text: text})
		}
	}
	return out
}

func (a *app) translatePointerButton(ev *C.xcb_generic_event_t, press bool) []platform.Event {
	b := (*C.xcb_button_press_event_t)(unsafe.Pointer(ev))
	button := platform.PointerNone
	switch b.detail {
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
			Position:  gfx.Point{X: float32(b.event_x), Y: float32(b.event_y)},
			Modifiers: modifiersFromState(uint16(b.state)),
		},
	}
}

func (a *app) translateMotion(ev *C.xcb_generic_event_t) []platform.Event {
	m := (*C.xcb_motion_notify_event_t)(unsafe.Pointer(ev))
	return []platform.Event{
		platform.EventPointer{
			Kind:      platform.PointerMove,
			Position:  gfx.Point{X: float32(m.event_x), Y: float32(m.event_y)},
			Modifiers: modifiersFromState(uint16(m.state)),
		},
	}
}

func (a *app) translateEnterLeave(ev *C.xcb_generic_event_t, enter bool) []platform.Event {
	e := (*C.xcb_enter_notify_event_t)(unsafe.Pointer(ev))
	kind := platform.PointerLeave
	if enter {
		kind = platform.PointerEnter
	}
	return []platform.Event{
		platform.EventPointer{
			Kind:      kind,
			Position:  gfx.Point{X: float32(e.event_x), Y: float32(e.event_y)},
			Modifiers: modifiersFromState(uint16(e.state)),
		},
	}
}

func (a *app) translateFocus(ev *C.xcb_generic_event_t, focused bool) []platform.Event {
	f := (*C.xcb_focus_in_event_t)(unsafe.Pointer(ev))
	win := a.lookupWindow(uint32(f.event))
	if win == nil {
		return nil
	}
	return []platform.Event{
		platform.EventWindowFocus{Window: win, Focused: focused},
	}
}

func (a *app) translateConfigure(ev *C.xcb_generic_event_t) []platform.Event {
	c := (*C.xcb_configure_notify_event_t)(unsafe.Pointer(ev))
	win := a.lookupWindow(uint32(c.window))
	if win == nil {
		return nil
	}
	win.width = int(c.width)
	win.height = int(c.height)
	if win.surface != nil {
		win.surface.resize(int(c.width), int(c.height))
	}
	return []platform.Event{
		platform.EventWindowResize{Window: win, Width: int(c.width), Height: int(c.height)},
	}
}

func (a *app) translateClientMessage(ev *C.xcb_generic_event_t) []platform.Event {
	cm := (*C.xcb_client_message_event_t)(unsafe.Pointer(ev))
	if C.uint32_t(C.lurpic_client_message_data32(cm)) != C.uint32_t(a.atomWMDelete) {
		return nil
	}
	win := a.lookupWindow(uint32(cm.window))
	if win == nil {
		return nil
	}
	return []platform.Event{
		platform.EventWindowClose{Window: win},
	}
}

func (a *app) handleSelectionRequest(ev *C.xcb_generic_event_t) {
	req := (*C.xcb_selection_request_event_t)(unsafe.Pointer(ev))
	c := a.clipboard
	if c == nil {
		return
	}

	var property C.xcb_atom_t = req.property
	if property == C.XCB_ATOM_NONE {
		property = req.target
	}

	if req.target == a.atomTargets {
		targets := []C.xcb_atom_t{a.atomUTF8String, a.atomTargets}
		C.xcb_change_property(a.conn, C.uint8_t(C.XCB_PROP_MODE_REPLACE), req.requestor, property, C.XCB_ATOM_ATOM, 32, C.uint32_t(len(targets)), unsafe.Pointer(&targets[0]))
	} else if req.target == a.atomUTF8String {
		c.app.mu.Lock()
		text := c.text
		c.app.mu.Unlock()
		cstr := C.CString(text)
		C.xcb_change_property(a.conn, C.uint8_t(C.XCB_PROP_MODE_REPLACE), req.requestor, property, req.target, 8, C.uint32_t(len(text)), unsafe.Pointer(cstr))
		C.free(unsafe.Pointer(cstr))
	}

	reply := C.xcb_selection_notify_event_t{
		response_type: C.uint8_t(C.XCB_SELECTION_NOTIFY),
		requestor:     req.requestor,
		selection:     req.selection,
		target:        req.target,
		property:      property,
		time:          req.time,
	}
	C.xcb_send_event(a.conn, 0, req.requestor, C.uint32_t(C.XCB_EVENT_MASK_NO_EVENT), (*C.char)(unsafe.Pointer(&reply)))
	C.xcb_flush(a.conn)
}

func (a *app) lookupWindow(id uint32) *window {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.windows[id]
}

func (s *shmSurface) resize(width, height int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if width <= 0 || height <= 0 {
		width = 1
		height = 1
	}
	if s.data != nil {
		C.lurpic_shmdt(s.data)
		s.data = nil
	}
	if s.shmid >= 0 {
		C.lurpic_shmrm(s.shmid)
		s.shmid = -1
	}
	stride := align(width*4, 4)
	size := stride * height
	shmid := C.lurpic_shmget(C.size_t(size))
	if shmid < 0 {
		return errors.New("linux platform: shmget resize failed")
	}
	data := C.lurpic_shmat(shmid)
	if C.lurpic_shmat_failed(data) != 0 || data == nil {
		C.lurpic_shmrm(shmid)
		return errors.New("linux platform: shmat resize failed")
	}
	s.shmid = shmid
	s.data = data
	s.buf = unsafe.Slice((*byte)(data), size)
	s.stride = stride
	s.width = width
	s.height = height
	s.seg = C.xcb_shm_seg_t(C.xcb_generate_id(s.conn))
	C.xcb_shm_attach(s.conn, s.seg, C.uint32_t(shmid), 0)
	C.xcb_flush(s.conn)
	return nil
}

func (a *app) DestroyClipboard() {
	if a == nil || a.clipboard == nil {
		return
	}
	a.clipboard.own = false
}

func (a *app) setClipboard(text string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.clipboard == nil {
		return
	}
	a.clipboard.text = text
	a.clipboard.own = true
}

func align(n, a int) int {
	if a <= 1 {
		return n
	}
	return int(math.Ceil(float64(n)/float64(a))) * a
}
