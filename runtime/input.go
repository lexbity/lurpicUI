package runtime

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/platform"
)

func (rt *Runtime) collectPlatformEvents() []platform.Event {
	if rt.platformApp == nil || rt.platformApp.Events() == nil {
		return nil
	}
	return rt.platformApp.Events().Poll()
}

func (rt *Runtime) handleWindowEvents(events []platform.Event) []platform.Event {
	if len(events) == 0 {
		return events[:0]
	}
	out := events[:0]
	for _, ev := range events {
		switch e := ev.(type) {
		case platform.LifecycleEvent:
			continue
		case platform.WindowEvent:
			continue
		case platform.EventWindowClose:
			rt.initiateShutdown()
		case platform.EventWindowResize:
			rt.handleResize(e.Width, e.Height)
		case platform.EventWindowFocus:
			if !e.Focused {
				rt.inputSystem.ClearPointerState()
				rt.ClearFocus()
			}
		case platform.ConfigurationChangedEvent:
			rt.handleConfigurationChanged(e)
		default:
			out = append(out, ev)
		}
	}
	return out
}

func (rt *Runtime) handleConfigurationChanged(e platform.ConfigurationChangedEvent) {
	// Recompute content scale based on density change.
	if e.Density > 0 {
		newScale := float32(e.Density) / 160.0 // Android: mdpi = 160 dpi = 1x
		if newScale != rt.contentScale && newScale > 0 {
			rt.contentScale = newScale
		}
	}
	// Mark the entire tree dirty so facets re-lay out for the new
	// configuration (orientation, density, fontScale, dark mode).
	rt.markTreeDirty(rt.root, facet.DirtyLayout|facet.DirtyProjection)
	if rt.frameTimer != nil {
		rt.frameTimer.RequestFrame()
	}
	rt.log.Info("runtime: configuration changed",
		"orientation", e.Orientation,
		"density", e.Density,
		"darkMode", e.UiModeNight,
		"fontScale", e.FontScale,
		"lang", e.Language,
	)
}
