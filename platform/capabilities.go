package platform

// WindowCapable marks apps that can create desktop-style windows.
type WindowCapable interface {
	NewWindow(opts WindowOptions) (Window, error)
}

// ClipboardCapable marks apps that expose a text clipboard.
type ClipboardCapable interface {
	Clipboard() Clipboard
}

// PointerCapable marks apps that can report pointer-device capabilities.
type PointerCapable interface {
	SupportsHover() bool
}

// IMECapable marks apps that can control the soft keyboard.
type IMECapable interface {
	ShowSoftKeyboard()
	HideSoftKeyboard()
}

// WindowCapableOf returns the window capability if app provides it.
func WindowCapableOf(app App) (WindowCapable, bool) {
	if app == nil {
		return nil, false
	}
	cap, ok := any(app).(WindowCapable)
	return cap, ok
}

// ClipboardCapableOf returns the clipboard capability if app provides it.
func ClipboardCapableOf(app App) (ClipboardCapable, bool) {
	if app == nil {
		return nil, false
	}
	cap, ok := any(app).(ClipboardCapable)
	return cap, ok
}

// PointerCapableOf returns the pointer capability if app provides it.
func PointerCapableOf(app App) (PointerCapable, bool) {
	if app == nil {
		return nil, false
	}
	cap, ok := any(app).(PointerCapable)
	return cap, ok
}

// IMECapableOf returns the IME capability if app provides it.
func IMECapableOf(app App) (IMECapable, bool) {
	if app == nil {
		return nil, false
	}
	cap, ok := any(app).(IMECapable)
	return cap, ok
}

// LifecycleCapable marks apps that handle Android lifecycle events.
// This is an Android-specific capability.
type LifecycleCapable interface {
	OnPause(func())
	OnResume(func())
	OnLowMemory(func())
	OnSurfaceLost(func())
	OnSurfaceCreated(func(Surface))
}

// LifecycleCapableOf returns the lifecycle capability if app provides it.
func LifecycleCapableOf(app App) (LifecycleCapable, bool) {
	if app == nil {
		return nil, false
	}
	cap, ok := any(app).(LifecycleCapable)
	return cap, ok
}
