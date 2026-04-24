package runtime

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

// ClearInputState resets pointer, hover, focus, and pending input state.
func (rt *Runtime) ClearInputState() {
	if rt == nil {
		return
	}
	if rt.inputSystem != nil {
		rt.inputSystem.ClearPointerState()
		rt.inputSystem.ClearFocus()
	}
	if rt.focusManager != nil {
		rt.focusManager.ClearFocus()
	}
	rt.pendingEvents = nil
}

// SetFocus grants focus to a concrete facet implementation.
func (rt *Runtime) SetFocus(target facet.FacetImpl) {
	if rt == nil || rt.focusManager == nil {
		return
	}
	rt.focusManager.SetFocus(target)
	rt.RequestFrame()
}

// ClearFocus removes the current focus target.
func (rt *Runtime) ClearFocus() {
	if rt == nil {
		return
	}
	if rt.focusManager != nil {
		rt.focusManager.ClearFocus()
	}
	if rt.inputSystem != nil {
		rt.inputSystem.ClearFocus()
	}
	rt.RequestFrame()
}

// FocusedID returns the currently focused facet ID, if any.
func (rt *Runtime) FocusedID() facet.FacetID {
	if rt == nil || rt.focusManager == nil {
		return 0
	}
	return rt.focusManager.Focused()
}

// ResizeWindow updates the platform window size if supported and marks the tree dirty.
func (rt *Runtime) ResizeWindow(width, height int) {
	if rt == nil {
		return
	}
	if rt.window != nil {
		if resizable, ok := rt.window.(interface{ Resize(int, int) }); ok {
			resizable.Resize(width, height)
		}
	}
	if rt.root != nil {
		rt.markTreeDirty(rt.root, facet.DirtyAll)
	}
	if rt.frameTimer != nil {
		rt.frameTimer.RequestFrame()
	}
}

// WindowSize returns the current window size.
func (rt *Runtime) WindowSize() (int, int) {
	if rt == nil {
		return 0, 0
	}
	return rt.windowSize()
}

// UpdateIMECursorRect allows callers to refresh the IME cursor rectangle after state changes.
func (rt *Runtime) UpdateIMECursorRect() {
	if rt == nil {
		return
	}
	rt.updateIMECursorRect()
}

// RootBounds returns the current root window size as a gfx.Size.
func (rt *Runtime) RootBounds() gfx.Size {
	w, h := rt.WindowSize()
	return gfx.Size{W: float32(w), H: float32(h)}
}
