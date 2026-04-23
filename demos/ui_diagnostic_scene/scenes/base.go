package scenes

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/ui_diagnostic_scene/scene"
)

// BaseScene provides common functionality for all diagnostic scenes.
// Embed this in concrete scene implementations.
type BaseScene struct {
	id          string
	displayName string
	description string
	families    []string
	root        facet.FacetImpl
	capability  scene.CapabilitySet
}

// NewBaseScene creates a new base scene with the given metadata.
func NewBaseScene(id, displayName, description string, families []string) BaseScene {
	return BaseScene{
		id:          id,
		displayName: displayName,
		description: description,
		families:    families,
		capability: scene.CapabilitySet{
			SupportsScreenshot:  true,
			SupportsSnapshot:    true,
			SupportsThemeSwitch: true,
			SupportsDensity:     true,
		},
	}
}

// SceneID returns the stable scene identifier.
func (b *BaseScene) SceneID() string {
	return b.id
}

// DisplayName returns the human-readable name.
func (b *BaseScene) DisplayName() string {
	return b.displayName
}

// Description returns the scene description.
func (b *BaseScene) Description() string {
	return b.description
}

// Families returns the mark families this scene validates.
func (b *BaseScene) Families() []string {
	return b.families
}

// BuildRoot returns the scene's facet tree.
func (b *BaseScene) BuildRoot() facet.FacetImpl {
	return b.root
}

// SetRoot sets the scene's root facet.
func (b *BaseScene) SetRoot(root facet.FacetImpl) {
	b.root = root
}

// Reset restores the scene to baseline by rebuilding the root.
// Override for custom reset behavior.
func (b *BaseScene) Reset() {
	// Default: rebuild root on next BuildRoot call
	b.root = nil
}

// ApplyTheme updates the scene for a new theme.
// Override for theme-aware scenes.
func (b *BaseScene) ApplyTheme(th theme.Context) {
	// Default: invalidate root to trigger rebuild
	if b.root != nil && b.root.Base() != nil {
		b.root.Base().Invalidate(facet.DirtyProjection)
	}
}

// ApplyDensity updates the scene for a new density scale.
// Override for density-aware scenes.
func (b *BaseScene) ApplyDensity(scale float32) {
	// Default: invalidate layout
	if b.root != nil && b.root.Base() != nil {
		b.root.Base().Invalidate(facet.DirtyLayout)
	}
}

// Capabilities returns the scene capability set.
func (b *BaseScene) Capabilities() scene.CapabilitySet {
	return b.capability
}

// SetCapability enables/disables a specific capability.
func (b *BaseScene) SetCapability(cap scene.CapabilitySet) {
	b.capability = cap
}

// ExportState returns serializable scene state.
// Override to export scene-specific state.
func (b *BaseScene) ExportState() map[string]any {
	return map[string]any{
		"scene_id": b.id,
	}
}

// ImportState restores scene state.
// Override to import scene-specific state.
func (b *BaseScene) ImportState(state map[string]any) {
	// Default: no state to import
}

// CreateDefaultRoot creates a simple stack layout root for scenes that need one.
func CreateDefaultRoot() *layout.StackLayout {
	return layout.NewStackLayout(layout.AlignStart)
}
