package studio

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/theme"
)

// Layer names for the studio demo's custom layers.
const (
	studioLayerModal       layout.LayerName = "studio.modal"
	studioLayerTooltip     layout.LayerName = "studio.tooltip"
	studioLayerAnchored    layout.LayerName = "studio.anchored"
	studioLayerTrigger     layout.LayerName = "studio.trigger"
	studioLayerRecipeRef                    = "anchored"
	studioTriggerRecipeRef                  = "trigger"
)

// studioLayerIDs are the facet layer IDs of the studio demo's custom layers,
// resolved once when the registry is installed so exhibits can mount overlays
// onto them.
type studioLayerIDs struct {
	modal    facet.LayerID
	tooltip  facet.LayerID
	anchored facet.LayerID
	trigger  facet.LayerID
}

// studioLayersFrom resolves the demo's custom layer IDs from a registry.
func studioLayersFrom(reg *layout.LayerRegistry) studioLayerIDs {
	if reg == nil {
		return studioLayerIDs{}
	}
	var out studioLayerIDs
	if desc, ok := reg.LookupName(studioLayerModal); ok {
		out.modal = facet.LayerID(desc.ID)
	}
	if desc, ok := reg.LookupName(studioLayerTooltip); ok {
		out.tooltip = facet.LayerID(desc.ID)
	}
	if desc, ok := reg.LookupName(studioLayerAnchored); ok {
		out.anchored = facet.LayerID(desc.ID)
	}
	if desc, ok := reg.LookupName(studioLayerTrigger); ok {
		out.trigger = facet.LayerID(desc.ID)
	}
	return out
}

// StudioLayerRegistry returns the runtime layer registry: the standard layers
// plus the studio demo's custom layers that carry the hit policies E2 and the
// anchor recipe E3 depend on. The app installs this via Config.Runtime.
func StudioLayerRegistry() (*layout.LayerRegistry, error) {
	b := layout.NewLayerRegistryBuilder()
	if err := b.RegisterStandardLayers(); err != nil {
		return nil, err
	}
	custom := []layout.LayerRegistration{
		{
			// The modal scrim blocks clicks on the base layer beneath it
			// (FR-layers / HitBlockBelow).
			Name:       studioLayerModal,
			Order:      7400,
			HitPolicy:  layout.HitBlockBelow,
			ClipPolicy: layout.ClipToParent,
		},
		{
			// The tooltip passes pointer input through to the base control
			// beneath it (FR-layers / HitPassThrough).
			Name:       studioLayerTooltip,
			Order:      2500,
			HitPolicy:  layout.HitPassThrough,
			ClipPolicy: layout.ClipToParent,
		},
		{
			// The anchored-popover layer resolves children with the anchor
			// layout recipe (E3: popovers track a moving trigger).
			Name:         studioLayerAnchored,
			Order:        4500,
			HitPolicy:    layout.HitNormal,
			ClipPolicy:   layout.ClipNone,
			LayoutRecipe: layout.LayerLayoutRecipeRef{Family: "studio", Name: studioLayerRecipeRef},
		},
		{
			// The trigger layer carries the E3 anchor source (a draggable
			// circle). It uses the free layout so the trigger can be dragged
			// anywhere while still exporting the anchors the popovers track.
			Name:         studioLayerTrigger,
			Order:        2200,
			HitPolicy:    layout.HitNormal,
			ClipPolicy:   layout.ClipNone,
			LayoutRecipe: layout.LayerLayoutRecipeRef{Family: "studio", Name: studioTriggerRecipeRef},
		},
	}
	for _, spec := range custom {
		if _, err := b.RegisterLayer(spec); err != nil {
			return nil, err
		}
	}
	return b.Freeze()
}

// StudioThemeContext returns the demo's resolved theme context with the studio
// layer recipes registered (the anchor recipe the studio.anchored layer
// resolves through).
func StudioThemeContext() theme.ResolvedContext {
	resolver := theme.NewThemeResolver()
	for _, scale := range theme.DefaultDensityScales(theme.Default().TokenSet()) {
		_ = resolver.RegisterDensityScale(scale)
	}
	_ = resolver.RegisterLayerLayoutRecipe(
		layout.LayerLayoutRecipeRef{Family: "studio", Name: studioLayerRecipeRef},
		func(theme.ResolvedContext) layout.ResolvedLayerLayoutRecipe {
			return layout.ResolvedLayerLayoutRecipe{PolicyKind: layout.LayerLayoutAnchor}
		},
	)
	_ = resolver.RegisterLayerLayoutRecipe(
		layout.LayerLayoutRecipeRef{Family: "studio", Name: studioTriggerRecipeRef},
		func(theme.ResolvedContext) layout.ResolvedLayerLayoutRecipe {
			return layout.ResolvedLayerLayoutRecipe{PolicyKind: layout.LayerLayoutFree}
		},
	)
	return theme.Default().WithResolver(resolver)
}
