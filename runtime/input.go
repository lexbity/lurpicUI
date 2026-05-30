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
		case platform.BackEvent:
			// Route back as an Escape key press + release to trigger
			// navigation-back behaviour in the facet system.
			rt.pendingEvents = append(rt.pendingEvents,
				platform.EventKey{Kind: platform.KeyPress, Key: platform.KeyEscape},
				platform.EventKey{Kind: platform.KeyRelease, Key: platform.KeyEscape},
			)
		case platform.VsyncEvent:
			if rt.frameTimer != nil {
				rt.frameTimer.Vsync(e.FrameTimeNanos)
			}
		case platform.AudioFocusEvent:
			rt.handleAudioFocusChange(e)
		case platform.ConfigurationChangedEvent:
			rt.handleConfigurationChanged(e)
		case platform.TrimMemoryEvent:
			rt.handleTrimMemory(e)
		default:
			out = append(out, ev)
		}
	}
	return out
}

func (rt *Runtime) handleAudioFocusChange(e platform.AudioFocusEvent) {
	// Forward audio focus changes to the audio subsystem if available.
	if a := platform.AudioCapableOf(rt.platformApp); a != nil {
		a.OnFocusChange(func(change platform.AudioFocusChange) {
			switch change {
			case platform.AudioFocusLoss:
				rt.log.Info("runtime: audio focus lost permanently")
			case platform.AudioFocusLossTransient:
				rt.log.Info("runtime: audio focus lost transiently — pausing audio")
			case platform.AudioFocusLossTransientCanDuck:
				rt.log.Info("runtime: audio focus lost — ducking audio")
			case platform.AudioFocusGain, platform.AudioFocusGainTransient:
				rt.log.Info("runtime: audio focus regained")
			}
		})
	}
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
