// android_hello is a test application for the Android build pipeline.
// It exercises the lurpicUI platform/android bridge and logs all lifecycle events.
//
// This app demonstrates:
// - NativeActivity lifecycle callbacks
// - Event queue processing
// - Platform.App interface usage
// - Touch input handling (Phase A6)
//
// Build with: lurpic build android
// Run with: lurpic run android
package main

import (
	"log"
	"os"

	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/platform/android"
)

func main() {
	log.Println("=== lurpicUI Android Hello ===")
	log.Println("Starting Android test application...")

	// Create the Android platform application
	app, err := android.NewApp()
	if err != nil {
		log.Fatalf("Failed to create Android app: %v", err)
	}
	defer app.Destroy()

	log.Println("Android app initialized successfully")
	log.Println("Monitoring lifecycle events...")

	// Get the event queue
	events := app.Events()

	// Event processing loop
	// In a real app, this would be driven by the runtime's main loop
	// For this test, we process events as they arrive
	processEvents(events)

	log.Println("Application shutting down")
}

// processEvents drains the event queue and logs all events
// This simulates what the runtime would do with platform events
func processEvents(eventQueue platform.EventQueue) {
	log.Println("Starting event processing loop")

	// Process events until we get a destroy event
	// In a real app, this would be the main loop
	for {
		// Poll for events (non-blocking)
		events := eventQueue.Poll()

		if len(events) > 0 {
			for _, event := range events {
				switch e := event.(type) {
				case platform.LifecycleEvent:
					handleLifecycleEvent(e)
				case platform.WindowEvent:
					handleWindowEvent(e)
				case platform.TouchEvent:
					handleTouchEvent(e)
				case platform.EventKey:
					handleKeyEvent(e)
				default:
					log.Printf("Unknown event type: %T", event)
				}

				// Exit on destroy lifecycle event
				if le, ok := event.(platform.LifecycleEvent); ok && le.Kind == platform.LifecycleDestroy {
					log.Println("Received destroy event, exiting event loop")
					return
				}
			}
		}

		// In a real app, we'd have proper timing here
		// For this test, we just continue polling
	}
}

func handleLifecycleEvent(e platform.LifecycleEvent) {
	switch e.Kind {
	case platform.LifecycleStart:
		log.Println("[LIFECYCLE] onStart - App becoming visible")
	case platform.LifecycleResume:
		log.Println("[LIFECYCLE] onResume - App gaining focus")
	case platform.LifecyclePause:
		log.Println("[LIFECYCLE] onPause - App losing focus")
	case platform.LifecycleStop:
		log.Println("[LIFECYCLE] onStop - App no longer visible")
	case platform.LifecycleDestroy:
		log.Println("[LIFECYCLE] onDestroy - App being destroyed")
	default:
		log.Printf("[LIFECYCLE] Unknown kind: %v", e.Kind)
	}
}

func handleWindowEvent(e platform.WindowEvent) {
	switch e.Kind {
	case platform.WindowCreated:
		log.Printf("[WINDOW] Window created: %dx%d", e.Width, e.Height)
	case platform.WindowResized:
		log.Printf("[WINDOW] Window resized: %dx%d", e.Width, e.Height)
	case platform.WindowDestroyed:
		log.Println("[WINDOW] Window destroyed")
	case platform.WindowFocusGained:
		log.Println("[WINDOW] Window gained focus")
	case platform.WindowFocusLost:
		log.Println("[WINDOW] Window lost focus")
	default:
		log.Printf("[WINDOW] Unknown kind: %v", e.Kind)
	}
}

func handleTouchEvent(e platform.TouchEvent) {
	switch e.Phase {
	case platform.TouchDown:
		log.Printf("[TOUCH] Down at (%.1f, %.1f) seq=%d", e.X, e.Y, e.SequenceID)
	case platform.TouchMove:
		log.Printf("[TOUCH] Move at (%.1f, %.1f) seq=%d", e.X, e.Y, e.SequenceID)
	case platform.TouchUp:
		log.Printf("[TOUCH] Up at (%.1f, %.1f) seq=%d", e.X, e.Y, e.SequenceID)
	case platform.TouchCancel:
		log.Printf("[TOUCH] Cancel seq=%d", e.SequenceID)
	default:
		log.Printf("[TOUCH] Unknown phase: %v", e.Phase)
	}
}

func handleKeyEvent(e platform.EventKey) {
	action := "pressed"
	switch e.Kind {
	case platform.KeyRelease:
		action = "released"
	case platform.KeyRepeat:
		action = "repeated"
	}
	log.Printf("[KEY] Key %s: %v", action, e.Key)
}

// init runs before main and can be used for additional setup
func init() {
	// Set up logging to both stdout and a file for debugging
	log.SetFlags(log.Ltime | log.Lmicroseconds)
	log.SetOutput(os.Stdout)
}
