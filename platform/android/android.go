//go:build android
// +build android

package android

import (
	"time"
	"unsafe"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/platform/android/internal/bridge"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/vulkan"
)

// EdgeInsets represents the safe-area insets for the current window.
type EdgeInsets struct {
	Top    int
	Bottom int
	Left   int
	Right  int
}

// App implements the platform.App interface for Android.
// It also implements platform.LifecycleCapable for lifecycle event handling.
type App struct {
	events            *bridge.EventQueue
	lifecycleHandlers lifecycleCallbacks
	currentSurface    platform.Surface
	currentInsets     EdgeInsets
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

// SupportsHover reports that Android supports hover interactions via stylus,
// mouse, and other pointer devices. The input system enables hover events
// when this returns true, allowing enter/leave/move tracking without a
// pressed button.
func (a *App) SupportsHover() bool {
	return true
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

	// Software present state (see surface_android.go). The pixel buffer is owned
	// by the ANativeWindow between Lock and Unlock.
	locked      bool
	bits        unsafe.Pointer
	strideBytes int
	geomSet     bool
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
	// Force the buffer geometry to be reapplied on the next Lock.
	s.geomSet = false
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
	// When the Vulkan renderer is already initialized (instance != 0) and
	// this is being called to recreate the surface after a pause/resume or
	// configuration change, use the recreate path which properly destroys
	// the old surface and rebuilds the swapchain.
	if instance != 0 && vulkan.InstanceHandle() != 0 {
		if err := vulkan.RecreateAndroidSurface(s.window, uint32(s.width), uint32(s.height)); err != nil {
			return 0, err
		}
		// Return the instance as a non-zero handle to signal success;
		// the Rust-side handles surface tracking internally.
		return instance, nil
	}
	return vulkan.CreateAndroidSurface(s.window, instance, uint32(s.width), uint32(s.height))
}

var _ platform.Surface = (*androidSurface)(nil)
var _ render.VulkanSurface = (*androidSurface)(nil)
var _ platform.IMECapable = (*App)(nil)

// SafeInsets returns the current window safe-area insets. The runtime layout
// system uses these values to avoid drawing under system bars, cutouts, and IME.
// Values are in pixels and are updated whenever the window insets change.
func (a *App) SafeInsets() EdgeInsets {
	if a == nil {
		return EdgeInsets{}
	}
	return a.currentInsets
}

// Surface returns the current native window surface, or nil if not yet available.
// On Android the surface is created by the system as part of the Activity lifecycle
// and delivered via a WindowCreated event through the bridge.
func (a *App) Surface() platform.Surface {
	return a.currentSurface
}

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

func (a *eventQueueAdapter) convert(e bridge.Event) platform.Event {
	return convertBridgeEvent(a.app, e)
}

func (a *eventQueueAdapter) Poll() []platform.Event {
	bridgeEvents := a.queue.Poll()
	events := make([]platform.Event, 0, len(bridgeEvents))
	for _, be := range bridgeEvents {
		if pe := a.convert(be); pe != nil {
			a.dispatchSideEffects(pe)
			events = append(events, pe)
		}
	}
	return events
}

func (a *eventQueueAdapter) Wait(timeout time.Duration) []platform.Event {
	bridgeEvents := a.queue.Wait()
	events := make([]platform.Event, 0, len(bridgeEvents))
	for _, be := range bridgeEvents {
		if pe := a.convert(be); pe != nil {
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
// The app parameter is used for side effects like storing window insets.
func convertBridgeEvent(a *App, e bridge.Event) platform.Event {
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
	case bridge.EventTypePointer:
		ptrKind := platform.PointerMove
		switch e.Action {
		case 0:
			ptrKind = platform.PointerPress
		case 1:
			ptrKind = platform.PointerRelease
		case 3:
			ptrKind = platform.PointerCancel
		}
		btn := platform.PointerLeft
		if e.Action == 0 || e.Action == 1 {
			switch e.ButtonState {
			case 2:
				btn = platform.PointerRight
			case 4:
				btn = platform.PointerMiddle
			}
		}
		return platform.EventPointer{
			Kind:      ptrKind,
			Position:  gfx.Point{X: e.X, Y: e.Y},
			Button:    btn,
			Modifiers: 0,
		}
	case bridge.EventTypeScroll:
		return platform.EventScroll{
			Position: gfx.Point{X: e.X, Y: e.Y},
			DeltaX:   e.Major,
			DeltaY:   e.Minor,
			Precise:  true,
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
	case bridge.EventTypeWindowInsets:
		if a != nil {
			a.currentInsets = EdgeInsets{
				Top:    int(e.InsetTop),
				Bottom: int(e.InsetBottom),
				Left:   int(e.InsetLeft),
				Right:  int(e.InsetRight),
			}
		}
		// Window insets are consumed by the App, they don't need to be
		// routed as a platform.Event to the runtime event loop.
		return nil
	case bridge.EventTypeConfigurationChanged:
		return platform.ConfigurationChangedEvent{
			Orientation:   int(e.Orientation),
			ScreenWidthDp: int(e.ScreenWidthDp),
			ScreenHeightDp: int(e.ScreenHeightDp),
			Density:       int(e.Density),
			UiModeNight:   e.UiModeNight != 0,
			FontScale:     e.FontScale,
			Language:      e.Language,
			Country:       e.Country,
		}
	default:
		return nil
	}
}
