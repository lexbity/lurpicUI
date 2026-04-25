//go:build android
// +build android

package android

import (
	"time"

	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/platform/android/internal/bridge"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/vulkan"
)

// App implements the platform.App interface for Android.
// It also implements platform.LifecycleCapable for lifecycle event handling.
type App struct {
	events            *bridge.EventQueue
	lifecycleHandlers lifecycleCallbacks
	currentSurface    platform.Surface
}

// lifecycleCallbacks stores registered lifecycle callbacks
type lifecycleCallbacks struct {
	onPause          []func()
	onResume         []func()
	onLowMemory      []func()
	onSurfaceLost    []func()
	onSurfaceCreated []func(platform.Surface)
}

// NewApp creates a new Android platform application instance.
// It initializes the native bridge and sets up the event queue.
func NewApp() (platform.App, error) {
	// Initialize the bridge
	bridge.Init()

	app := &App{
		events: bridge.GetEventQueue(),
	}

	return app, nil
}

// OnPause registers a callback for Android onPause events.
func (a *App) OnPause(f func()) {
	a.lifecycleHandlers.onPause = append(a.lifecycleHandlers.onPause, f)
}

// OnResume registers a callback for Android onResume events.
func (a *App) OnResume(f func()) {
	a.lifecycleHandlers.onResume = append(a.lifecycleHandlers.onResume, f)
}

// OnLowMemory registers a callback for Android onLowMemory events.
func (a *App) OnLowMemory(f func()) {
	a.lifecycleHandlers.onLowMemory = append(a.lifecycleHandlers.onLowMemory, f)
}

// OnSurfaceLost registers a callback for surface destruction events.
func (a *App) OnSurfaceLost(f func()) {
	a.lifecycleHandlers.onSurfaceLost = append(a.lifecycleHandlers.onSurfaceLost, f)
}

// OnSurfaceCreated registers a callback for surface creation events.
func (a *App) OnSurfaceCreated(f func(platform.Surface)) {
	a.lifecycleHandlers.onSurfaceCreated = append(a.lifecycleHandlers.onSurfaceCreated, f)
}

// SupportsHover reports whether Android hover interactions should be enabled.
func (a *App) SupportsHover() bool {
	return false
}

// ShowSoftKeyboard asks the native bridge to display the soft keyboard.
func (a *App) ShowSoftKeyboard() {
	bridge.ShowSoftKeyboard()
}

// HideSoftKeyboard asks the native bridge to dismiss the soft keyboard.
func (a *App) HideSoftKeyboard() {
	bridge.HideSoftKeyboard()
}

// dispatchLifecycleEvent delivers lifecycle events to registered callbacks.
func (a *App) dispatchLifecycleEvent(kind platform.LifecycleKind) {
	if a == nil {
		return
	}
	switch kind {
	case platform.LifecyclePause:
		for _, f := range a.lifecycleHandlers.onPause {
			f()
		}
	case platform.LifecycleResume:
		for _, f := range a.lifecycleHandlers.onResume {
			f()
		}
	case platform.LifecycleLowMemory:
		for _, f := range a.lifecycleHandlers.onLowMemory {
			f()
		}
	}
}

// dispatchWindowEvent delivers window events to registered callbacks.
func (a *App) dispatchWindowEvent(e platform.WindowEvent) {
	if a == nil {
		return
	}
	switch e.Kind {
	case platform.WindowCreated:
		surf := newAndroidSurface(e.Window, e.Width, e.Height)
		a.currentSurface = surf
		for _, f := range a.lifecycleHandlers.onSurfaceCreated {
			f(surf)
		}
	case platform.WindowResized:
		if a.currentSurface != nil {
			a.currentSurface.Resize(e.Width, e.Height)
			for _, f := range a.lifecycleHandlers.onSurfaceCreated {
				f(a.currentSurface)
			}
		}
	case platform.WindowDestroyed:
		a.currentSurface = nil
		for _, f := range a.lifecycleHandlers.onSurfaceLost {
			f()
		}
	}
}

type androidSurface struct {
	window uintptr
	width  int
	height int
	scale  float32
}

func newAndroidSurface(window uintptr, width, height int) platform.Surface {
	if width <= 0 {
		width = 1
	}
	if height <= 0 {
		height = 1
	}
	return &androidSurface{window: window, width: width, height: height, scale: 1}
}

func (s *androidSurface) Size() (width, height int) { return s.width, s.height }

func (s *androidSurface) Resize(width, height int) {
	if width <= 0 {
		width = 1
	}
	if height <= 0 {
		height = 1
	}
	s.width = width
	s.height = height
}

func (s *androidSurface) Scale() float32 {
	if s == nil || s.scale <= 0 {
		return 1
	}
	return s.scale
}

func (s *androidSurface) VulkanInstanceExtensions() []string {
	return []string{"VK_KHR_surface", "VK_KHR_android_surface"}
}

func (s *androidSurface) CreateVulkanSurface(instance uintptr) (uintptr, error) {
	if s == nil || s.window == 0 {
		return 0, nil
	}
	return vulkan.CreateAndroidSurface(s.window, instance, uint32(s.width), uint32(s.height))
}

var _ platform.Surface = (*androidSurface)(nil)
var _ render.VulkanSurface = (*androidSurface)(nil)
var _ platform.IMECapable = (*App)(nil)

// Events returns the platform event queue.
func (a *App) Events() platform.EventQueue {
	return &eventQueueAdapter{app: a, queue: a.events}
}

// Destroy cleans up the Android application.
func (a *App) Destroy() {
	a.events.Close()
}

// eventQueueAdapter adapts the bridge.EventQueue to platform.EventQueue.
type eventQueueAdapter struct {
	app   *App
	queue *bridge.EventQueue
}

func (a *eventQueueAdapter) Push(e platform.Event) {
	// This adapter is one-way: Android -> Go
	// Events from Android come through the bridge, not this method
}

func (a *eventQueueAdapter) Poll() []platform.Event {
	bridgeEvents := a.queue.Poll()
	events := make([]platform.Event, 0, len(bridgeEvents))
	for _, be := range bridgeEvents {
		if pe := convertBridgeEvent(be); pe != nil {
			a.dispatchSideEffects(pe)
			events = append(events, pe)
		}
	}
	return events
}

func (a *eventQueueAdapter) Wait(timeout time.Duration) []platform.Event {
	// TODO: Implement timeout-based waiting
	bridgeEvents := a.queue.Wait()
	events := make([]platform.Event, 0, len(bridgeEvents))
	for _, be := range bridgeEvents {
		if pe := convertBridgeEvent(be); pe != nil {
			a.dispatchSideEffects(pe)
			events = append(events, pe)
		}
	}
	return events
}

func (a *eventQueueAdapter) dispatchSideEffects(e platform.Event) {
	if a == nil || a.app == nil || e == nil {
		return
	}
	switch ev := e.(type) {
	case platform.LifecycleEvent:
		a.app.dispatchLifecycleEvent(ev.Kind)
	case platform.WindowEvent:
		a.app.dispatchWindowEvent(ev)
	}
}

// convertBridgeEvent converts a bridge event to a platform event.
func convertBridgeEvent(e bridge.Event) platform.Event {
	switch e.Type {
	case bridge.EventTypeStart:
		return platform.LifecycleEvent{Kind: platform.LifecycleStart}
	case bridge.EventTypeResume:
		return platform.LifecycleEvent{Kind: platform.LifecycleResume}
	case bridge.EventTypePause:
		return platform.LifecycleEvent{Kind: platform.LifecyclePause}
	case bridge.EventTypeStop:
		return platform.LifecycleEvent{Kind: platform.LifecycleStop}
	case bridge.EventTypeDestroy:
		return platform.LifecycleEvent{Kind: platform.LifecycleDestroy}
	case bridge.EventTypeLowMemory:
		return platform.LifecycleEvent{Kind: platform.LifecycleLowMemory}
	case bridge.EventTypeNativeWindowCreated:
		return platform.WindowEvent{Kind: platform.WindowCreated, Window: uintptr(e.Window), Width: e.Width, Height: e.Height}
	case bridge.EventTypeNativeWindowResized:
		return platform.WindowEvent{Kind: platform.WindowResized, Window: uintptr(e.Window), Width: e.Width, Height: e.Height}
	case bridge.EventTypeNativeWindowDestroyed:
		return platform.WindowEvent{Kind: platform.WindowDestroyed, Window: uintptr(e.Window)}
	case bridge.EventTypeWindowFocusChanged:
		kind := platform.WindowFocusLost
		if e.Focused {
			kind = platform.WindowFocusGained
		}
		return platform.WindowEvent{Kind: kind, Window: uintptr(e.Window)}
	case bridge.EventTypeTouch:
		return platform.TouchEvent{
			SequenceID: uint64(e.PointerID),
			Phase:      platform.TouchPhase(e.Phase),
			X:          e.X,
			Y:          e.Y,
			Pressure:   e.Pressure,
		}
	case bridge.EventTypeKey:
		kind := platform.KeyPress
		switch e.Action {
		case 1:
			kind = platform.KeyRelease
		case 2:
			kind = platform.KeyRepeat
		}
		return platform.EventKey{
			Kind:      kind,
			Key:       e.Key,
			Modifiers: e.Modifiers,
		}
	case bridge.EventTypeIMECompose:
		return platform.EventIMECompose{Text: e.Text, CursorPos: e.CursorPos}
	case bridge.EventTypeIMECommit:
		return platform.EventIMECommit{Text: e.Text}
	default:
		return nil
	}
}
