//go:build android
// +build android

// Package bridge provides the Go-side implementation of the Android native bridge.
//
// This package exports functions that are called from C via JNI, and provides
// a thread-safe event queue for delivering Android lifecycle and input events
// to the lurpicUI runtime.
package bridge

import (
	"sync"
	"time"
	"unsafe"

	"codeburg.org/lexbit/lurpicui/platform"
)

// #cgo LDFLAGS: -landroid -llog
// #include <android/native_activity.h>
// #include <android/native_window.h>
// #include <android/input.h>
// #include <android/keycodes.h>
// #include <stdlib.h>
// void bridgeShowSoftKeyboard(void);
// void bridgeHideSoftKeyboard(void);
// void bridgeRequestPermission(const char* permission, int requestCode);
// int bridgeCheckPermission(const char* permission);
// int bridgeIsPermissionDeclared(const char* permission);
// void bridgeSetExtractionProgress(float progress);
import "C"

// EventType represents the type of Android event.
type EventType int

const (
	EventTypeStart EventType = iota
	EventTypeResume
	EventTypePause
	EventTypeStop
	EventTypeDestroy
	EventTypeWindowFocusChanged
	EventTypeNativeWindowCreated
	EventTypeNativeWindowDestroyed
	EventTypeNativeWindowResized
	EventTypeInputQueueCreated
	EventTypeInputQueueDestroyed
	EventTypeLowMemory
	EventTypeTouch
	EventTypePointer
	EventTypeScroll
	EventTypeKey
	EventTypeIMECompose
	EventTypeIMECommit
	EventTypeConfigurationChanged
	EventTypeWindowInsets
	EventTypeAudioFocusChange
	EventTypeVsync
	EventTypeSavedState
	EventTypeBackInvoked
	EventTypeWindowMetricsChanged
)

// Event represents an Android lifecycle or input event.
type Event struct {
	Type      EventType
	Timestamp time.Time
	// Lifecycle fields
	Activity unsafe.Pointer
	Window   unsafe.Pointer
	Width    int
	Height   int
	Queue    unsafe.Pointer
	Focused  bool
	// Touch fields
	PointerID   int32
	Phase       TouchPhase
	X, Y        float32
	Pressure    float32
	Major       float32
	Minor       float32
	Source      int32
	DeviceID    int32
	ToolType    int32
	ButtonState int32
	EventTime   int64
	// Key fields
	KeyCode   int32
	Action    int32
	MetaState int32
	Key       platform.Key
	Modifiers platform.ModifierKeys
	// IME fields
	Text      string
	CursorPos int
	// Window inset fields
	InsetTop    int32
	InsetBottom int32
	InsetLeft   int32
	InsetRight  int32
	CutoutLeft  int32
	CutoutTop   int32
	CutoutRight int32
	CutoutBottom int32
	// Audio focus field
	FocusChange int32
	// Vsync field
	FrameTimeNanos int64
	// Saved state fields (for process death restoration)
	SavedState []byte
	// Configuration fields
	Orientation   int32
	ScreenWidthDp int32
	ScreenHeightDp int32
	Density       int32
	UiModeNight   int32
	FontScale     float32
	Language      string
	Country       string
}

// TouchPhase represents the phase of a touch event.
type TouchPhase int

const (
	TouchDown TouchPhase = iota
	TouchMove
	TouchUp
	TouchCancel
)

// EventQueue is a thread-safe queue for Android events.
type EventQueue struct {
	mu     sync.Mutex
	cond   *sync.Cond
	events []Event
	closed bool
}

// NewEventQueue creates a new event queue.
func NewEventQueue() *EventQueue {
	q := &EventQueue{}
	q.cond = sync.NewCond(&q.mu)
	return q
}

// Push adds an event to the queue.
func (q *EventQueue) Push(e Event) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return
	}

	e.Timestamp = time.Now()
	q.events = append(q.events, e)
	q.cond.Broadcast()
}

// Poll removes and returns all available events.
func (q *EventQueue) Poll() []Event {
	q.mu.Lock()
	defer q.mu.Unlock()

	events := q.events
	q.events = nil
	return events
}

// Wait blocks until events are available or the queue is closed.
func (q *EventQueue) Wait() []Event {
	q.mu.Lock()
	defer q.mu.Unlock()

	for len(q.events) == 0 && !q.closed {
		q.cond.Wait()
	}

	events := q.events
	q.events = nil
	return events
}

// Close marks the queue as closed and unblocks waiters.
func (q *EventQueue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.closed = true
	q.cond.Broadcast()
}

// IsClosed returns true if the queue has been closed.
func (q *EventQueue) IsClosed() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.closed
}

// Global event queue instance
var (
	globalQueue     *EventQueue
	globalQueueOnce sync.Once
	permissionMu    sync.RWMutex
	permissionHook  func(requestCode int32, granted bool, permanent bool)
	assetManagerMu  sync.RWMutex
	assetManagerPtr unsafe.Pointer
	activityMu      sync.RWMutex
	activityPtr     unsafe.Pointer
)

// setAssetManager records the AAssetManager* supplied by the NativeActivity.
func setAssetManager(p unsafe.Pointer) {
	assetManagerMu.Lock()
	assetManagerPtr = p
	assetManagerMu.Unlock()
}

// setActivity records the ANativeActivity* supplied by onCreate.
func setActivity(p unsafe.Pointer) {
	activityMu.Lock()
	activityPtr = p
	activityMu.Unlock()
}

// GetActivity returns the ANativeActivity* captured from the NativeActivity
// onCreate callback. It returns nil before the activity has been created.
func GetActivity() unsafe.Pointer {
	activityMu.RLock()
	defer activityMu.RUnlock()
	return activityPtr
}

// GetAssetManager returns the AAssetManager* captured from the NativeActivity
// for JNI asset access. It returns nil before the activity has been created.
func GetAssetManager() unsafe.Pointer {
	assetManagerMu.RLock()
	defer assetManagerMu.RUnlock()
	return assetManagerPtr
}

// GetEventQueue returns the singleton event queue.
func GetEventQueue() *EventQueue {
	globalQueueOnce.Do(func() {
		globalQueue = NewEventQueue()
	})
	return globalQueue
}

//export goANativeActivityOnCreate
func goANativeActivityOnCreate(activity *C.ANativeActivity, savedState unsafe.Pointer, savedStateSize C.size_t) {
	event := Event{
		Type:     EventTypeStart,
		Activity: unsafe.Pointer(activity),
	}
	GetEventQueue().Push(event)

	// Capture the AAssetManager* and ANativeActivity* so app.Asset can read
	// bundled APK assets and the extraction pipeline can access storage paths.
	if activity != nil {
		setAssetManager(unsafe.Pointer(activity.assetManager))
		setActivity(unsafe.Pointer(activity))
	}

	// Log that we received the create event
	androidLogInfo("ANativeActivity_onCreate called, activity=%p", unsafe.Pointer(activity))
}

//export goOnStart
func goOnStart(activity *C.ANativeActivity) {
	event := Event{
		Type:     EventTypeStart,
		Activity: unsafe.Pointer(activity),
	}
	GetEventQueue().Push(event)
	androidLogInfo("onStart called")
}

//export goOnResume
func goOnResume(activity *C.ANativeActivity) {
	event := Event{
		Type:     EventTypeResume,
		Activity: unsafe.Pointer(activity),
	}
	GetEventQueue().Push(event)
	androidLogInfo("onResume called")
}

//export goOnPause
func goOnPause(activity *C.ANativeActivity) {
	event := Event{
		Type:     EventTypePause,
		Activity: unsafe.Pointer(activity),
	}
	GetEventQueue().Push(event)
	androidLogInfo("onPause called")
}

//export goOnStop
func goOnStop(activity *C.ANativeActivity) {
	event := Event{
		Type:     EventTypeStop,
		Activity: unsafe.Pointer(activity),
	}
	GetEventQueue().Push(event)
	androidLogInfo("onStop called")
}

//export goOnDestroy
func goOnDestroy(activity *C.ANativeActivity) {
	event := Event{
		Type:     EventTypeDestroy,
		Activity: unsafe.Pointer(activity),
	}
	GetEventQueue().Push(event)
	GetEventQueue().Close()
	androidLogInfo("onDestroy called")
}

//export goOnWindowFocusChanged
func goOnWindowFocusChanged(activity *C.ANativeActivity, focused C.int) {
	event := Event{
		Type:     EventTypeWindowFocusChanged,
		Activity: unsafe.Pointer(activity),
		Focused:  focused != 0,
	}
	GetEventQueue().Push(event)
	androidLogInfo("onWindowFocusChanged: %v", focused != 0)
}

//export goOnNativeWindowCreated
func goOnNativeWindowCreated(activity *C.ANativeActivity, window *C.ANativeWindow) {
	width := int(C.ANativeWindow_getWidth(window))
	height := int(C.ANativeWindow_getHeight(window))
	event := Event{
		Type:     EventTypeNativeWindowCreated,
		Activity: unsafe.Pointer(activity),
		Window:   unsafe.Pointer(window),
		Width:    width,
		Height:   height,
	}
	GetEventQueue().Push(event)
	androidLogInfo("onNativeWindowCreated: %p (%dx%d)", unsafe.Pointer(window), width, height)
}

//export goOnNativeWindowDestroyed
func goOnNativeWindowDestroyed(activity *C.ANativeActivity, window *C.ANativeWindow) {
	event := Event{
		Type:     EventTypeNativeWindowDestroyed,
		Activity: unsafe.Pointer(activity),
		Window:   unsafe.Pointer(window),
	}
	GetEventQueue().Push(event)
	androidLogInfo("onNativeWindowDestroyed: %p", unsafe.Pointer(window))
}

//export goOnNativeWindowResized
func goOnNativeWindowResized(activity *C.ANativeActivity, window *C.ANativeWindow) {
	width := int(C.ANativeWindow_getWidth(window))
	height := int(C.ANativeWindow_getHeight(window))
	event := Event{
		Type:     EventTypeNativeWindowResized,
		Activity: unsafe.Pointer(activity),
		Window:   unsafe.Pointer(window),
		Width:    width,
		Height:   height,
	}
	GetEventQueue().Push(event)
	androidLogInfo("onNativeWindowResized: %p (%dx%d)", unsafe.Pointer(window), width, height)
}

//export goOnInputQueueCreated
func goOnInputQueueCreated(activity *C.ANativeActivity, queue *C.AInputQueue) {
	event := Event{
		Type:     EventTypeInputQueueCreated,
		Activity: unsafe.Pointer(activity),
		Queue:    unsafe.Pointer(queue),
	}
	GetEventQueue().Push(event)
	androidLogInfo("onInputQueueCreated: %p", unsafe.Pointer(queue))
}

//export goOnInputQueueDestroyed
func goOnInputQueueDestroyed(activity *C.ANativeActivity, queue *C.AInputQueue) {
	event := Event{
		Type:     EventTypeInputQueueDestroyed,
		Activity: unsafe.Pointer(activity),
		Queue:    unsafe.Pointer(queue),
	}
	GetEventQueue().Push(event)
	androidLogInfo("onInputQueueDestroyed: %p", unsafe.Pointer(queue))
}

//export goOnLowMemory
func goOnLowMemory(activity *C.ANativeActivity) {
	event := Event{
		Type:     EventTypeLowMemory,
		Activity: unsafe.Pointer(activity),
	}
	GetEventQueue().Push(event)
	androidLogInfo("onLowMemory called")
}

//export goDeliverTouchEvent
func goDeliverTouchEvent(pointerID C.int32_t, phase C.int32_t, x C.float, y C.float,
	pressure C.float, major C.float, minor C.float,
	source C.int32_t, deviceID C.int32_t, toolType C.int32_t,
	buttonState C.int32_t, eventTime C.int64_t) {
	event := Event{
		Type:        EventTypeTouch,
		PointerID:   int32(pointerID),
		Phase:       TouchPhase(phase),
		X:           float32(x),
		Y:           float32(y),
		Pressure:    float32(pressure),
		Major:       float32(major),
		Minor:       float32(minor),
		Source:      int32(source),
		DeviceID:    int32(deviceID),
		ToolType:    int32(toolType),
		ButtonState: int32(buttonState),
		EventTime:   int64(eventTime),
	}
	GetEventQueue().Push(event)
}

//export goDeliverPointerEvent
func goDeliverPointerEvent(pointerID C.int32_t, action C.int32_t, x C.float, y C.float,
	pressure C.float, size C.float,
	source C.int32_t, deviceID C.int32_t, toolType C.int32_t,
	buttonState C.int32_t, eventTime C.int64_t) {
	event := Event{
		Type:        EventTypePointer,
		PointerID:   int32(pointerID),
		Action:      int32(action),
		X:           float32(x),
		Y:           float32(y),
		Pressure:    float32(pressure),
		Major:       float32(size),
		Minor:       float32(size),
		Source:      int32(source),
		DeviceID:    int32(deviceID),
		ToolType:    int32(toolType),
		ButtonState: int32(buttonState),
		EventTime:   int64(eventTime),
	}
	GetEventQueue().Push(event)
}

//export goDeliverScrollEvent
func goDeliverScrollEvent(x C.float, y C.float, hScroll C.float, vScroll C.float,
	source C.int32_t, deviceID C.int32_t, eventTime C.int64_t) {
	event := Event{
		Type:      EventTypeScroll,
		X:         float32(x),
		Y:         float32(y),
		Major:     float32(hScroll),
		Minor:     float32(vScroll),
		Source:    int32(source),
		DeviceID:  int32(deviceID),
		EventTime: int64(eventTime),
	}
	GetEventQueue().Push(event)
}

//export goDeliverKeyEvent
func goDeliverKeyEvent(keyCode C.int32_t, action C.int32_t, metaState C.int32_t,
	source C.int32_t, deviceID C.int32_t, eventTime C.int64_t) {
	event := Event{
		Type:      EventTypeKey,
		KeyCode:   int32(keyCode),
		Action:    int32(action),
		MetaState: int32(metaState),
		Key:       mapAndroidKeyCode(keyCode),
		Modifiers: mapAndroidMetaState(metaState),
		Source:    int32(source),
		DeviceID:  int32(deviceID),
		EventTime: int64(eventTime),
	}
	GetEventQueue().Push(event)
}

//export goDeliverIMECompose
func goDeliverIMECompose(text *C.char, cursorPos C.int32_t) {
	contents := ""
	if text != nil {
		contents = C.GoString(text)
	}
	event := Event{
		Type:      EventTypeIMECompose,
		Text:      contents,
		CursorPos: int(cursorPos),
	}
	GetEventQueue().Push(event)
}

//export goDeliverIMECommit
func goDeliverIMECommit(text *C.char) {
	contents := ""
	if text != nil {
		contents = C.GoString(text)
	}
	event := Event{
		Type: EventTypeIMECommit,
		Text: contents,
	}
	GetEventQueue().Push(event)
}

// Saved state for process death restoration.
var (
	savedStateMu sync.RWMutex
	savedState   []byte
)

//export goGetSavedState
func goGetSavedState(outData **C.char, outLen *C.int32_t) C.int {
	savedStateMu.RLock()
	defer savedStateMu.RUnlock()
	if len(savedState) == 0 {
		return 0
	}
	data := C.CBytes(savedState)
	*outData = (*C.char)(data)
	*outLen = C.int32_t(len(savedState))
	return 1
}

//export goSetSavedState
func goSetSavedState(data *C.char, len C.int32_t) {
	if data == nil || len <= 0 {
		return
	}
	savedStateMu.Lock()
	savedState = C.GoBytes(unsafe.Pointer(data), len)
	savedStateMu.Unlock()
}

// GetSavedState returns the most recently saved view state, or nil.
func GetSavedState() []byte {
	savedStateMu.RLock()
	defer savedStateMu.RUnlock()
	if len(savedState) == 0 {
		return nil
	}
	out := make([]byte, len(savedState))
	copy(out, savedState)
	return out
}

// SetSavedState stores view state that will be returned by GetSavedState
// and delivered to the runtime for restoration.
func SetSavedState(data []byte) {
	savedStateMu.Lock()
	if len(data) == 0 {
		savedState = nil
	} else {
		savedState = make([]byte, len(data))
		copy(savedState, data)
	}
	savedStateMu.Unlock()
}

// ClearSavedState removes any stored saved state.
func ClearSavedState() {
	savedStateMu.Lock()
	savedState = nil
	savedStateMu.Unlock()
}

//export goDeliverVsync
func goDeliverVsync(frameTimeNanos C.int64_t) {
	event := Event{
		Type:           EventTypeVsync,
		FrameTimeNanos: int64(frameTimeNanos),
	}
	GetEventQueue().Push(event)
}

//export goDeliverBackInvoked
func goDeliverBackInvoked() {
	GetEventQueue().Push(Event{Type: EventTypeBackInvoked})
}

//export goDeliverWindowMetricsChanged
func goDeliverWindowMetricsChanged(width C.int32_t, height C.int32_t) {
	event := Event{
		Type:   EventTypeWindowMetricsChanged,
		Width:  int(width),
		Height: int(height),
	}
	GetEventQueue().Push(event)
}

//export goDeliverAudioFocusChange
func goDeliverAudioFocusChange(focusChange C.int32_t) {
	event := Event{
		Type:        EventTypeAudioFocusChange,
		FocusChange: int32(focusChange),
	}
	GetEventQueue().Push(event)
}

//export goDeliverWindowInsets
func goDeliverWindowInsets(top C.int32_t, bottom C.int32_t, left C.int32_t, right C.int32_t,
	cutoutLeft C.int32_t, cutoutTop C.int32_t, cutoutRight C.int32_t, cutoutBottom C.int32_t) {
	event := Event{
		Type:         EventTypeWindowInsets,
		InsetTop:     int32(top),
		InsetBottom:  int32(bottom),
		InsetLeft:    int32(left),
		InsetRight:   int32(right),
		CutoutLeft:   int32(cutoutLeft),
		CutoutTop:    int32(cutoutTop),
		CutoutRight:  int32(cutoutRight),
		CutoutBottom: int32(cutoutBottom),
	}
	GetEventQueue().Push(event)
}

//export goDeliverConfigurationChanged
func goDeliverConfigurationChanged(orientation C.int32_t, screenWidthDp C.int32_t,
	screenHeightDp C.int32_t, density C.int32_t, uiModeNight C.int32_t,
	fontScale C.float, language *C.char, country *C.char) {
	event := Event{
		Type:          EventTypeConfigurationChanged,
		Orientation:   int32(orientation),
		ScreenWidthDp: int32(screenWidthDp),
		ScreenHeightDp: int32(screenHeightDp),
		Density:       int32(density),
		UiModeNight:   int32(uiModeNight),
		FontScale:     float32(fontScale),
	}
	if language != nil {
		event.Language = C.GoString(language)
	}
	if country != nil {
		event.Country = C.GoString(country)
	}
	GetEventQueue().Push(event)
}

//export goDeliverPermissionResult
func goDeliverPermissionResult(requestCode C.int32_t, granted C.int32_t, permanent C.int32_t) {
	permissionMu.RLock()
	hook := permissionHook
	permissionMu.RUnlock()
	if hook == nil {
		return
	}
	hook(int32(requestCode), granted != 0, permanent != 0)
}

// SetPermissionResultHandler registers the callback for permission results.
func SetPermissionResultHandler(handler func(requestCode int32, granted bool, permanent bool)) {
	permissionMu.Lock()
	permissionHook = handler
	permissionMu.Unlock()
}

func mapAndroidMetaState(metaState C.int32_t) platform.ModifierKeys {
	var mods platform.ModifierKeys
	if metaState&C.AMETA_SHIFT_ON != 0 {
		mods |= platform.ModShift
	}
	if metaState&C.AMETA_CTRL_ON != 0 {
		mods |= platform.ModControl
	}
	if metaState&C.AMETA_ALT_ON != 0 {
		mods |= platform.ModAlt
	}
	if metaState&C.AMETA_META_ON != 0 {
		mods |= platform.ModSuper
	}
	return mods
}

func mapAndroidKeyCode(keyCode C.int32_t) platform.Key {
	switch keyCode {
	case C.AKEYCODE_A:
		return platform.KeyA
	case C.AKEYCODE_B:
		return platform.KeyB
	case C.AKEYCODE_C:
		return platform.KeyC
	case C.AKEYCODE_D:
		return platform.KeyD
	case C.AKEYCODE_E:
		return platform.KeyE
	case C.AKEYCODE_F:
		return platform.KeyF
	case C.AKEYCODE_G:
		return platform.KeyG
	case C.AKEYCODE_H:
		return platform.KeyH
	case C.AKEYCODE_I:
		return platform.KeyI
	case C.AKEYCODE_J:
		return platform.KeyJ
	case C.AKEYCODE_K:
		return platform.KeyK
	case C.AKEYCODE_L:
		return platform.KeyL
	case C.AKEYCODE_M:
		return platform.KeyM
	case C.AKEYCODE_N:
		return platform.KeyN
	case C.AKEYCODE_O:
		return platform.KeyO
	case C.AKEYCODE_P:
		return platform.KeyP
	case C.AKEYCODE_Q:
		return platform.KeyQ
	case C.AKEYCODE_R:
		return platform.KeyR
	case C.AKEYCODE_S:
		return platform.KeyS
	case C.AKEYCODE_T:
		return platform.KeyT
	case C.AKEYCODE_U:
		return platform.KeyU
	case C.AKEYCODE_V:
		return platform.KeyV
	case C.AKEYCODE_W:
		return platform.KeyW
	case C.AKEYCODE_X:
		return platform.KeyX
	case C.AKEYCODE_Y:
		return platform.KeyY
	case C.AKEYCODE_Z:
		return platform.KeyZ
	case C.AKEYCODE_DPAD_LEFT:
		return platform.KeyLeft
	case C.AKEYCODE_DPAD_RIGHT:
		return platform.KeyRight
	case C.AKEYCODE_DPAD_UP:
		return platform.KeyUp
	case C.AKEYCODE_DPAD_DOWN:
		return platform.KeyDown
	case C.AKEYCODE_MOVE_HOME:
		return platform.KeyHome
	case C.AKEYCODE_MOVE_END:
		return platform.KeyEnd
	case C.AKEYCODE_PAGE_UP:
		return platform.KeyPageUp
	case C.AKEYCODE_PAGE_DOWN:
		return platform.KeyPageDown
	case C.AKEYCODE_ESCAPE:
		return platform.KeyEscape
	case C.AKEYCODE_ENTER:
		return platform.KeyEnter
	case C.AKEYCODE_SPACE:
		return platform.KeySpace
	case C.AKEYCODE_TAB:
		return platform.KeyTab
	case C.AKEYCODE_DEL:
		return platform.KeyBackspace
	default:
		return platform.KeyUnknown
	}
}

// ShowSoftKeyboard asks the Java bridge to display the soft keyboard.
func ShowSoftKeyboard() {
	C.bridgeShowSoftKeyboard()
}

// HideSoftKeyboard asks the Java bridge to dismiss the soft keyboard.
func HideSoftKeyboard() {
	C.bridgeHideSoftKeyboard()
}

// RequestPermission asks the Java bridge to show the permission dialog.
func RequestPermission(permission string, requestCode int32) error {
	cPermission := C.CString(permission)
	defer C.free(unsafe.Pointer(cPermission))
	C.bridgeRequestPermission(cPermission, C.int(requestCode))
	return nil
}

// CheckPermission queries the Java bridge for the current permission state.
func CheckPermission(permission string) bool {
	cPermission := C.CString(permission)
	defer C.free(unsafe.Pointer(cPermission))
	return C.bridgeCheckPermission(cPermission) != 0
}

// IsPermissionDeclared reports whether the permission exists in the app manifest.
func IsPermissionDeclared(permission string) bool {
	cPermission := C.CString(permission)
	defer C.free(unsafe.Pointer(cPermission))
	return C.bridgeIsPermissionDeclared(cPermission) != 0
}

// SetExtractionProgress reports extraction progress to the Java UI layer
// so the splash screen can update.
func SetExtractionProgress(progress float32) {
	C.bridgeSetExtractionProgress(C.float(progress))
}

// androidLogInfo logs an info message via Android's log system.
// In a full implementation, this would call __android_log_print via C.
func androidLogInfo(format string, args ...interface{}) {
	// For now, this is a placeholder. The actual logging happens in C.
	// The C bridge logs before calling these Go functions.
	_ = format
	_ = args
}

// StartVsync enables Choreographer vsync callbacks. Should be called when
// the app is visible and actively rendering.
func StartVsync() {
	C.bridgeVsyncStart()
}

// StopVsync disables Choreographer vsync callbacks. Should be called when
// the app is paused or no longer visible.
func StopVsync() {
	C.bridgeVsyncStop()
}

// Init initializes the bridge. Called from the Android platform package.
func Init() {
	// Ensure the event queue is created
	_ = GetEventQueue()
}
