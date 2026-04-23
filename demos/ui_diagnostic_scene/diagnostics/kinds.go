package diagnostics

// OverlayKind identifies a diagnostic visualization overlay.
// These are stable identifiers that the app uses to request specific
// diagnostic visualizations without depending on engine internals.
type OverlayKind int

const (
	// OverlayOff disables all overlays.
	OverlayOff OverlayKind = iota

	// OverlayBounds shows facet arranged bounds.
	OverlayBounds

	// OverlayDirty shows dirty facets and invalidation reasons.
	OverlayDirty

	// OverlayHitRegions shows hit test regions.
	OverlayHitRegions

	// OverlayAnchors shows anchor points and attachment lines.
	OverlayAnchors

	// OverlayFocus shows focus chain and active focus owner.
	OverlayFocus

	// OverlayLayers shows layer bounds and clip regions.
	OverlayLayers

	// OverlayTiming shows frame timing breakdown.
	OverlayTiming

	// OverlayAll enables all available overlays.
	OverlayAll
)

// String returns the human-readable name of the overlay kind.
func (k OverlayKind) String() string {
	switch k {
	case OverlayOff:
		return "Off"
	case OverlayBounds:
		return "Bounds"
	case OverlayDirty:
		return "Dirty"
	case OverlayHitRegions:
		return "HitRegions"
	case OverlayAnchors:
		return "Anchors"
	case OverlayFocus:
		return "Focus"
	case OverlayLayers:
		return "Layers"
	case OverlayTiming:
		return "Timing"
	case OverlayAll:
		return "All"
	default:
		return "Unknown"
	}
}

// IsValid reports whether the overlay kind is a known value.
func (k OverlayKind) IsValid() bool {
	return k >= OverlayOff && k <= OverlayAll
}

// CanCombine reports whether this overlay kind can be combined with others.
// Some overlays are mutually exclusive (like Off and All).
func (k OverlayKind) CanCombine() bool {
	return k != OverlayOff && k != OverlayAll
}

// EventCategory identifies the semantic category of a diagnostic event.
type EventCategory int

const (
	// EventPointer covers pointer routing events.
	EventPointer EventCategory = iota

	// EventKeyboard covers keyboard routing events.
	EventKeyboard

	// EventText covers text input and IME events.
	EventText

	// EventStore covers store mutations.
	EventStore

	// EventSignal covers signal emissions.
	EventSignal

	// EventScene covers scene-local warnings.
	EventScene

	// EventRender covers render and diagnostic changes.
	EventRender

	// EventLifecycle covers facet lifecycle events.
	EventLifecycle
)

// String returns the human-readable name of the event category.
func (c EventCategory) String() string {
	switch c {
	case EventPointer:
		return "Pointer"
	case EventKeyboard:
		return "Keyboard"
	case EventText:
		return "Text"
	case EventStore:
		return "Store"
	case EventSignal:
		return "Signal"
	case EventScene:
		return "Scene"
	case EventRender:
		return "Render"
	case EventLifecycle:
		return "Lifecycle"
	default:
		return "Unknown"
	}
}

// Severity indicates the importance of a diagnostic event.
type Severity int

const (
	// SeverityDebug is for verbose diagnostic output.
	SeverityDebug Severity = iota

	// SeverityInfo is for normal operational events.
	SeverityInfo

	// SeverityWarning indicates potential issues.
	SeverityWarning

	// SeverityError indicates failures.
	SeverityError
)

// String returns the human-readable severity level.
func (s Severity) String() string {
	switch s {
	case SeverityDebug:
		return "Debug"
	case SeverityInfo:
		return "Info"
	case SeverityWarning:
		return "Warning"
	case SeverityError:
		return "Error"
	default:
		return "Unknown"
	}
}
