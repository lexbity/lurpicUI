package structure

// ShellVariant describes the coarse shell arrangement used by a demo.
type ShellVariant string

const (
	ShellVariantDesktopDense    ShellVariant = "desktop-dense"
	ShellVariantDesktopCompact  ShellVariant = "desktop-compact"
	ShellVariantTabletSplit     ShellVariant = "tablet-split"
	ShellVariantMobilePortrait  ShellVariant = "mobile-portrait"
	ShellVariantMobileLandscape ShellVariant = "mobile-landscape"
)

// NavigationMode describes the primary navigation affordance for a shell.
type NavigationMode string

const (
	NavigationSidebar NavigationMode = "sidebar"
	NavigationDrawer  NavigationMode = "drawer"
	NavigationTabs    NavigationMode = "tabs"
)

// ResponsiveLayout captures shell-level presentation preferences for a viewport.
type ResponsiveLayout struct {
	Profile Profile
	Variant ShellVariant

	Navigation NavigationMode

	OuterPadding          float32
	PanelGap              float32
	MinHitTarget          float32
	ShowHoverHints        bool
	ShowKeyboardShortcuts bool

	SecondaryCollapsed bool
	OptionalCollapsed  bool
}

// ResponsiveLayoutForViewport classifies the shell presentation for a viewport.
func ResponsiveLayoutForViewport(v Viewport, caps Capabilities) ResponsiveLayout {
	return ResponsiveLayoutForProfile(ProfileForViewport(v, caps), caps)
}

// ResponsiveLayoutForProfile returns presentation defaults for the given profile.
func ResponsiveLayoutForProfile(profile Profile, caps Capabilities) ResponsiveLayout {
	layout := ResponsiveLayout{
		Profile: profile,

		OuterPadding:          16,
		PanelGap:              8,
		MinHitTarget:          44,
		ShowHoverHints:        caps.Hover && !caps.Touch,
		ShowKeyboardShortcuts: caps.Keyboard || !caps.Touch,
	}

	switch profile {
	case ProfileDesktopDense:
		layout.Variant = ShellVariantDesktopDense
		layout.Navigation = NavigationSidebar
		layout.OuterPadding = 20
		layout.PanelGap = 12
		layout.MinHitTarget = 32
	case ProfileDesktopCompact:
		layout.Variant = ShellVariantDesktopCompact
		layout.Navigation = NavigationSidebar
		layout.OuterPadding = 16
		layout.PanelGap = 8
		layout.MinHitTarget = 36
	case ProfileTabletLike:
		layout.Variant = ShellVariantTabletSplit
		layout.Navigation = NavigationTabs
		layout.OuterPadding = 16
		layout.PanelGap = 10
		layout.MinHitTarget = 48
		layout.SecondaryCollapsed = false
		layout.OptionalCollapsed = true
	case ProfileMobilePortrait:
		layout.Variant = ShellVariantMobilePortrait
		layout.Navigation = NavigationDrawer
		layout.OuterPadding = 20
		layout.PanelGap = 12
		layout.MinHitTarget = 56
		layout.SecondaryCollapsed = true
		layout.OptionalCollapsed = true
	case ProfileMobileLandscape:
		layout.Variant = ShellVariantMobileLandscape
		layout.Navigation = NavigationTabs
		layout.OuterPadding = 18
		layout.PanelGap = 10
		layout.MinHitTarget = 52
		layout.SecondaryCollapsed = false
		layout.OptionalCollapsed = true
	default:
		layout.Variant = ShellVariantDesktopCompact
		layout.Navigation = NavigationSidebar
	}

	if caps.Touch {
		layout.ShowHoverHints = false
	}
	if caps.IME || caps.Keyboard {
		layout.ShowKeyboardShortcuts = true
	}

	return layout
}

// TouchHitTarget returns the minimum recommended touch target for the profile.
func TouchHitTarget(profile Profile) float32 {
	switch profile {
	case ProfileDesktopDense:
		return 32
	case ProfileDesktopCompact:
		return 36
	case ProfileTabletLike:
		return 48
	case ProfileMobilePortrait:
		return 56
	case ProfileMobileLandscape:
		return 52
	default:
		return 44
	}
}

// ShellSpacing returns a practical shell spacing for the profile.
func ShellSpacing(profile Profile) float32 {
	switch profile {
	case ProfileDesktopDense:
		return 12
	case ProfileDesktopCompact:
		return 8
	case ProfileTabletLike:
		return 10
	case ProfileMobilePortrait:
		return 12
	case ProfileMobileLandscape:
		return 10
	default:
		return 8
	}
}

// ShowHoverHints reports whether hover hints should be shown.
func ShowHoverHints(caps Capabilities) bool {
	return caps.Hover && !caps.Touch
}

// ShowKeyboardShortcuts reports whether keyboard shortcut hints should be shown.
func ShowKeyboardShortcuts(caps Capabilities) bool {
	return caps.Keyboard || (!caps.Touch && !caps.IME)
}

// OrderedSurfaceIDs returns surface IDs sorted by their shell priority.
func (m TargetModel) OrderedSurfaceIDs() []string {
	if len(m.Surfaces) == 0 {
		return nil
	}
	ids := make([]string, 0, len(m.Surfaces))
	ids = append(ids, m.PrimarySurfaceIDs()...)
	ids = append(ids, m.SecondarySurfaceIDs()...)
	ids = append(ids, m.OptionalSurfaceIDs()...)
	if len(ids) == 0 {
		return nil
	}
	return ids
}

// VisibleSurfaceIDs returns the ordered list of surface IDs that should remain visible.
func (m TargetModel) VisibleSurfaceIDs(layout ResponsiveLayout) []string {
	ids := m.OrderedSurfaceIDs()
	if len(ids) == 0 {
		return nil
	}

	switch layout.Variant {
	case ShellVariantMobilePortrait:
		return m.PrimarySurfaceIDs()
	case ShellVariantMobileLandscape:
		visible := m.PrimarySurfaceIDs()
		visible = append(visible, m.SecondarySurfaceIDs()...)
		return visible
	case ShellVariantTabletSplit:
		visible := m.PrimarySurfaceIDs()
		visible = append(visible, m.SecondarySurfaceIDs()...)
		return visible
	default:
		return ids
	}
}

// SurfaceNavigator models a mobile-friendly navigation set for a target model.
type SurfaceNavigator struct {
	ActiveID string
	IDs      []string
}

// NewSurfaceNavigator builds a navigation model from the target model and layout.
func NewSurfaceNavigator(model TargetModel, layout ResponsiveLayout) SurfaceNavigator {
	ids := model.VisibleSurfaceIDs(layout)
	if len(ids) == 0 {
		ids = model.OrderedSurfaceIDs()
	}
	if len(ids) == 0 {
		return SurfaceNavigator{}
	}
	active := ids[0]
	return SurfaceNavigator{ActiveID: active, IDs: append([]string(nil), ids...)}
}

// Contains reports whether the navigator includes the given surface id.
func (n SurfaceNavigator) Contains(id string) bool {
	for _, candidate := range n.IDs {
		if candidate == id {
			return true
		}
	}
	return false
}

// Next returns the next id after active, wrapping around.
func (n SurfaceNavigator) Next(active string) string {
	if len(n.IDs) == 0 {
		return ""
	}
	for idx, candidate := range n.IDs {
		if candidate == active {
			return n.IDs[(idx+1)%len(n.IDs)]
		}
	}
	return n.IDs[0]
}

// Prev returns the previous id before active, wrapping around.
func (n SurfaceNavigator) Prev(active string) string {
	if len(n.IDs) == 0 {
		return ""
	}
	for idx, candidate := range n.IDs {
		if candidate == active {
			if idx == 0 {
				return n.IDs[len(n.IDs)-1]
			}
			return n.IDs[idx-1]
		}
	}
	return n.IDs[0]
}

// PanelToggleState models a shared collapse/expand state for secondary panels.
type PanelToggleState struct {
	Expanded bool
}

// Collapse marks the panel as collapsed.
func (s *PanelToggleState) Collapse() {
	if s != nil {
		s.Expanded = false
	}
}

// Expand marks the panel as expanded.
func (s *PanelToggleState) Expand() {
	if s != nil {
		s.Expanded = true
	}
}

// Toggle flips the panel state.
func (s *PanelToggleState) Toggle() {
	if s != nil {
		s.Expanded = !s.Expanded
	}
}

// Visible reports whether the panel should be visible for the current layout.
func (s PanelToggleState) Visible(defaultVisible bool) bool {
	if s.Expanded {
		return true
	}
	return defaultVisible
}
