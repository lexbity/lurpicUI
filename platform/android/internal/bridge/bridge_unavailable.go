//go:build !android || (android && !cgo)
// +build !android

package bridge

import (
	"errors"
	"sync"
	"time"
	"unsafe"

	"codeburg.org/lexbit/lurpicui/platform"
)

// Non-Android fallback implementations.
// This keeps the package importable on any platform while returning
// explicit platform errors for Android-only operations.

type EventType int
type TouchPhase int

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

const (
	TouchDown TouchPhase = iota
	TouchMove
	TouchUp
	TouchCancel
)

type Event struct {
	Type        EventType
	Kind        int // Event subtype (lifecycle kind, window kind, key kind, etc.)
	Timestamp   time.Time
	Activity    unsafe.Pointer
	Window      unsafe.Pointer
	Width       int
	Height      int
	Queue       unsafe.Pointer
	Focused     bool
	PointerID   int32
	SequenceID  uint64 // For touch events
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
	KeyCode     int32
	Key         platform.Key
	Modifiers   platform.ModifierKeys
	Action      int32
	MetaState   int32
	Text        string
	CursorPos   int
	FocusChange int32
	FrameTimeNanos int64
	SavedState  []byte
	InsetTop    int32
	InsetBottom int32
	InsetLeft   int32
	InsetRight  int32
	CutoutLeft  int32
	CutoutTop   int32
	CutoutRight int32
	CutoutBottom int32
	Orientation   int32
	ScreenWidthDp int32
	ScreenHeightDp int32
	Density       int32
	UiModeNight   int32
	FontScale     float32
	Language      string
	Country       string
}

// EventQueue is the non-Android event queue.
type EventQueue struct {
	mu     sync.Mutex
	cond   *sync.Cond
	events []Event
	closed bool
}

func NewEventQueue() *EventQueue {
	q := &EventQueue{}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func (q *EventQueue) Push(e Event) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if !q.closed {
		q.events = append(q.events, e)
		q.cond.Broadcast()
	}
}

func (q *EventQueue) Poll() []Event {
	q.mu.Lock()
	defer q.mu.Unlock()
	events := q.events
	q.events = nil
	return events
}

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

func (q *EventQueue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	q.cond.Broadcast()
}

func (q *EventQueue) IsClosed() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.closed
}

var (
	globalQueue     *EventQueue
	globalQueueOnce sync.Once
)

func GetEventQueue() *EventQueue {
	globalQueueOnce.Do(func() {
		globalQueue = NewEventQueue()
	})
	return globalQueue
}

func Init() {
	// No-op on non-Android
}

func SetPermissionResultHandler(func(requestCode int32, granted bool, permanent bool)) {}

func RequestPermission(permission string, requestCode int32) error {
	return ErrNotAndroid
}

func CheckPermission(permission string) bool {
	return false
}

func IsPermissionDeclared(permission string) bool {
	return false
}

// GetAssetManager returns nil on non-Android platforms.
func GetAssetManager() unsafe.Pointer {
	return nil
}

// Saved state — works on all platforms for testing.
var (
	testSavedStateMu sync.Mutex
	testSavedState   []byte
)

func GetSavedState() []byte {
	testSavedStateMu.Lock()
	defer testSavedStateMu.Unlock()
	if len(testSavedState) == 0 {
		return nil
	}
	out := make([]byte, len(testSavedState))
	copy(out, testSavedState)
	return out
}

func SetSavedState(data []byte) {
	testSavedStateMu.Lock()
	if len(data) == 0 {
		testSavedState = nil
	} else {
		testSavedState = make([]byte, len(data))
		copy(testSavedState, data)
	}
	testSavedStateMu.Unlock()
}

func ClearSavedState() {
	testSavedStateMu.Lock()
	testSavedState = nil
	testSavedStateMu.Unlock()
}

// ErrNotAndroid is returned when Android-specific functions are called on non-Android platforms.
var ErrNotAndroid = errors.New("android bridge not available on this platform")
