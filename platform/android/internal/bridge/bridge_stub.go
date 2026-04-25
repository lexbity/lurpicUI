//go:build !android
// +build !android

package bridge

import (
	"errors"
	"sync"
	"time"
	"unsafe"

	"codeburg.org/lexbit/lurpicui/platform"
)

// Stub implementations for non-Android builds.
// This allows the package to be imported on any platform,
// though the actual functionality only works on Android.

type EventType int
type TouchPhase int

// Event category constants for type dispatching
const (
	EventLifecycle EventType = iota
	EventWindow
	EventTouch
	EventKey
)

// Legacy event type constants (for backward compatibility)
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
	EventTypeKey
	EventTypeIMECompose
	EventTypeIMECommit
)

const (
	TouchDown TouchPhase = iota
	TouchMove
	TouchUp
	TouchCancel
)

type Event struct {
	Type       EventType
	Kind       int // Event subtype (lifecycle kind, window kind, key kind, etc.)
	Timestamp  time.Time
	Activity   unsafe.Pointer
	Window     unsafe.Pointer
	Width      int
	Height     int
	Queue      unsafe.Pointer
	Focused    bool
	PointerID  int32
	SequenceID uint64 // For touch events
	Phase      TouchPhase
	X, Y       float32
	Pressure   float32
	Major      float32
	Minor      float32
	KeyCode    int32
	Key        platform.Key
	Modifiers  platform.ModifierKeys
	Action     int32
	MetaState  int32
	Text       string
	CursorPos  int
}

// EventQueue is a stub for non-Android builds.
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
// On Android, this returns the AAssetManager* for JNI asset access.
func GetAssetManager() unsafe.Pointer {
	return nil
}

// ErrNotAndroid is returned when Android-specific functions are called on non-Android platforms.
var ErrNotAndroid = errors.New("android bridge not available on this platform")
