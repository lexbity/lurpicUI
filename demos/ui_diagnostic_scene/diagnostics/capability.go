package diagnostics

// SceneCapabilitySummary provides a view of scene capabilities for diagnostics.
// This helps the UI show relevant controls for each scene.
type SceneCapabilitySummary struct {
	// SceneID is the stable identifier for the scene.
	SceneID string

	// SceneName is the human-readable display name.
	SceneName string

	// HasStressControls indicates if the scene provides stress testing UI.
	HasStressControls bool

	// SupportsScreenshot indicates if the scene can export screenshots.
	SupportsScreenshot bool

	// SupportsSnapshot indicates if the scene can export state snapshots.
	SupportsSnapshot bool

	// SupportsThemeSwitch indicates if the scene handles theme changes.
	SupportsThemeSwitch bool

	// SupportsDensity indicates if the scene handles density changes.
	SupportsDensity bool

	// HasCustomLogs indicates if the scene provides custom log output.
	HasCustomLogs bool

	// Families lists the mark families this scene validates.
	Families []string

	// Description explains what this scene tests.
	Description string
}

// HasFamily reports whether this scene covers the given mark family.
func (s SceneCapabilitySummary) HasFamily(family string) bool {
	for _, f := range s.Families {
		if f == family {
			return true
		}
	}
	return false
}

// ActiveOverlays tracks which overlays are currently enabled for a scene.
type ActiveOverlays struct {
	// EnabledOverlays is a set of active overlay kinds.
	EnabledOverlays map[OverlayKind]bool

	// SceneID identifies which scene these overlays apply to.
	SceneID string
}

// NewActiveOverlays creates a new ActiveOverlays for the given scene.
func NewActiveOverlays(sceneID string) ActiveOverlays {
	return ActiveOverlays{
		EnabledOverlays: make(map[OverlayKind]bool),
		SceneID:         sceneID,
	}
}

// IsEnabled reports whether the given overlay kind is active.
func (a ActiveOverlays) IsEnabled(kind OverlayKind) bool {
	if a.EnabledOverlays == nil {
		return false
	}
	return a.EnabledOverlays[kind]
}

// SetEnabled sets the enabled state for an overlay kind.
func (a *ActiveOverlays) SetEnabled(kind OverlayKind, enabled bool) {
	if a.EnabledOverlays == nil {
		a.EnabledOverlays = make(map[OverlayKind]bool)
	}
	a.EnabledOverlays[kind] = enabled
}

// Toggle toggles the enabled state for an overlay kind.
func (a *ActiveOverlays) Toggle(kind OverlayKind) bool {
	newState := !a.IsEnabled(kind)
	a.SetEnabled(kind, newState)
	return newState
}

// AnyEnabled reports whether any overlay is currently active.
func (a ActiveOverlays) AnyEnabled() bool {
	for _, enabled := range a.EnabledOverlays {
		if enabled {
			return true
		}
	}
	return false
}

// EnabledList returns a slice of all enabled overlay kinds.
func (a ActiveOverlays) EnabledList() []OverlayKind {
	var list []OverlayKind
	for kind, enabled := range a.EnabledOverlays {
		if enabled {
			list = append(list, kind)
		}
	}
	return list
}

// Clear disables all overlays.
func (a *ActiveOverlays) Clear() {
	a.EnabledOverlays = make(map[OverlayKind]bool)
}
