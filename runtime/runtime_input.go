package runtime

import (
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
		default:
			out = append(out, ev)
		}
	}
	return out
}
