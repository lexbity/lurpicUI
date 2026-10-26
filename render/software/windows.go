package software

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/render"
)

// ConfigureWindows prepares independent authoritative outputs and diff/raster
// state. Textures are immutable generations shared by the render owner.
func (r *SoftwareRenderer) ConfigureWindows(targets []render.WindowTarget) error {
	if len(targets) == 0 || len(targets) > render.MaxFrameWindows {
		return fmt.Errorf("software renderer: invalid window count %d", len(targets))
	}
	seen := make(map[string]bool, len(targets))
	windows := make(map[string]*SoftwareRenderer, len(targets)-1)
	for _, target := range targets {
		if target.ID == "" || seen[target.ID] || target.Surface == nil {
			for _, window := range windows {
				window.Destroy()
			}
			return fmt.Errorf("software renderer: invalid window target %q", target.ID)
		}
		seen[target.ID] = true
		if target.ID == render.PrimaryWindow {
			continue
		}
		window := NewSoftwareRenderer()
		if err := window.Initialize(target.Surface); err != nil {
			for _, existing := range windows {
				existing.Destroy()
			}
			return fmt.Errorf("software renderer: window %q: %w", target.ID, err)
		}
		window.windowID = target.ID
		window.texBackend = r.texBackend
		windows[target.ID] = window
	}
	if !seen[render.PrimaryWindow] {
		for _, window := range windows {
			window.Destroy()
		}
		return fmt.Errorf("software renderer: primary window missing")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, window := range r.windows {
		window.Destroy()
	}
	r.windows = windows
	return nil
}

func (r *SoftwareRenderer) SubmitWindow(id string, frame *render.Frame) error {
	if frame == nil || frame.WindowID() != id {
		return fmt.Errorf("render: packet window does not match target %q", id)
	}
	if id == render.PrimaryWindow {
		return r.Submit(frame)
	}
	r.mu.RLock()
	window := r.windows[id]
	r.mu.RUnlock()
	if window == nil {
		return fmt.Errorf("software renderer: window %q is unavailable", id)
	}
	return window.Submit(frame)
}

var _ render.WindowBackend = (*SoftwareRenderer)(nil)
