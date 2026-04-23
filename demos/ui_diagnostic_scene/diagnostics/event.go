package diagnostics

import (
	"time"
)

// Event represents a single diagnostic event in the app.
// Events are semantically meaningful and stay usable under load.
type Event struct {
	// Timestamp is when the event occurred.
	Timestamp time.Time

	// Category is the semantic category of the event.
	Category EventCategory

	// Severity indicates the importance of the event.
	Severity Severity

	// Message is a human-readable description.
	Message string

	// Source identifies where the event originated (e.g., facet ID, scene ID).
	Source string

	// Data contains optional structured data relevant to the event.
	// The structure depends on the Category.
	Data map[string]any
}

// EventDataPointer contains data for pointer events.
type EventDataPointer struct {
	Point      [2]float32 // x, y in screen space
	Button     int
	Pressed    bool
	Released   bool
	TargetID   string
	TargetType string
}

// EventDataKeyboard contains data for keyboard events.
type EventDataKeyboard struct {
	KeyCode    int
	KeyName    string
	Modifiers  int // bitfield
	Pressed    bool
	Repeated   bool
	TargetID   string
}

// EventDataStore contains data for store mutation events.
type EventDataStore struct {
	StoreName   string
	MutationType string // "set", "update", "clear"
	Version     uint64
}

// EventDataSignal contains data for signal emission events.
type EventDataSignal struct {
	SignalName    string
	SubscriberCount int
}

// EventDataScene contains data for scene warnings.
type EventDataScene struct {
	SceneID   string
	WarningType string
}

// EventDataRender contains data for render changes.
type EventDataRender struct {
	FrameNumber   uint64
	DirtyFacetCount int
	BatchCount      int
}

// EventDataLifecycle contains data for facet lifecycle events.
type EventDataLifecycle struct {
	FacetID   string
	FacetType string
	Transition string // "attach", "detach", "activate", "deactivate"
}

// NewEvent creates a new diagnostic event with the current timestamp.
func NewEvent(category EventCategory, severity Severity, message, source string) Event {
	return Event{
		Timestamp: time.Now(),
		Category:  category,
		Severity:  severity,
		Message:   message,
		Source:    source,
		Data:      nil,
	}
}

// NewEventWithData creates a new diagnostic event with structured data.
func NewEventWithData(category EventCategory, severity Severity, message, source string, data map[string]any) Event {
	return Event{
		Timestamp: time.Now(),
		Category:  category,
		Severity:  severity,
		Message:   message,
		Source:    source,
		Data:      data,
	}
}

// FormatTimestamp returns a formatted timestamp string.
func (e Event) FormatTimestamp() string {
	return e.Timestamp.Format("15:04:05.000")
}

// HasData reports whether this event has structured data attached.
func (e Event) HasData() bool {
	return len(e.Data) > 0
}

// IsImportant reports whether this event should be highlighted.
func (e Event) IsImportant() bool {
	return e.Severity >= SeverityWarning
}
