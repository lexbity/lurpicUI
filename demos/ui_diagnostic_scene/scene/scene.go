package scene

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/theme"
)

// Scene is the contract for a diagnostic scene.
// Each scene validates specific mark families and failure modes.
type Scene interface {
	// SceneID returns the stable scene identifier (e.g., "layout-torture")
	SceneID() string

	// DisplayName returns the human-readable name for navigation
	DisplayName() string

	// BuildRoot constructs the scene's facet tree. Returns nil if not yet implemented.
	BuildRoot() facet.FacetImpl

	// Reset restores the scene to its baseline state
	Reset()

	// ApplyTheme updates the scene for a new theme
	ApplyTheme(theme.Context)

	// ApplyDensity updates the scene for a new density scale
	ApplyDensity(scale float32)

	// Capabilities describes what this scene can do
	Capabilities() CapabilitySet

	// ExportState returns serializable scene state for bug reports
	ExportState() map[string]any

	// ImportState restores scene state from a previous export
	ImportState(state map[string]any)
}

// CapabilitySet describes scene capabilities
type CapabilitySet struct {
	HasStressControls   bool
	SupportsScreenshot  bool
	SupportsSnapshot    bool
	SupportsThemeSwitch bool
	SupportsDensity     bool
	HasCustomLogs       bool
}

// Definition registers a scene in the registry
type Definition struct {
	ID          string
	DisplayName string
	Description string
	Families    []string // Mark families this scene validates
	Factory     func() Scene
}

// Registry holds all available scenes
type Registry struct {
	defs map[string]Definition
	ids  []string // preserve registration order
}

// NewRegistry creates an empty scene registry
func NewRegistry() *Registry {
	return &Registry{
		defs: make(map[string]Definition),
	}
}

// Register adds a scene definition to the registry
func (r *Registry) Register(def Definition) {
	if _, exists := r.defs[def.ID]; exists {
		// Already registered, ignore duplicate
		return
	}
	r.defs[def.ID] = def
	r.ids = append(r.ids, def.ID)
}

// Get retrieves a scene definition by ID
func (r *Registry) Get(id string) (Definition, bool) {
	def, ok := r.defs[id]
	return def, ok
}

// GetAll returns all registered scene definitions in order
func (r *Registry) GetAll() []Definition {
	result := make([]Definition, 0, len(r.ids))
	for _, id := range r.ids {
		result = append(result, r.defs[id])
	}
	return result
}

// Count returns the number of registered scenes
func (r *Registry) Count() int {
	return len(r.ids)
}

// Create instantiates a scene by ID
func (r *Registry) Create(id string) (Scene, bool) {
	def, ok := r.defs[id]
	if !ok {
		return nil, false
	}
	if def.Factory == nil {
		return nil, false
	}
	return def.Factory(), true
}
