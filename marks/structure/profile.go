package structure

import (
	"fmt"
	"sort"
)

// Profile describes the coarse device/layout mode the demo shells should use.
type Profile int

const (
	ProfileDesktopDense Profile = iota
	ProfileDesktopCompact
	ProfileTabletLike
	ProfileMobilePortrait
	ProfileMobileLandscape
)

func (p Profile) String() string {
	switch p {
	case ProfileDesktopDense:
		return "desktop-dense"
	case ProfileDesktopCompact:
		return "desktop-compact"
	case ProfileTabletLike:
		return "tablet-like"
	case ProfileMobilePortrait:
		return "mobile-portrait"
	case ProfileMobileLandscape:
		return "mobile-landscape"
	default:
		return "unknown"
	}
}

// Capabilities captures the platform interaction capabilities used by the classifier.
type Capabilities struct {
	Touch    bool
	Hover    bool
	Keyboard bool
	IME      bool
}

// Viewport captures the pixel-space window or surface size used for profile selection.
type Viewport struct {
	Width  int
	Height int
}

// ProfileForViewport classifies the viewport into a coarse layout profile.
func ProfileForViewport(v Viewport, caps Capabilities) Profile {
	w := clampDimension(v.Width)
	h := clampDimension(v.Height)
	short := min(w, h)
	long := max(w, h)

	if caps.Touch {
		if short >= 1100 && long >= 1600 {
			return ProfileTabletLike
		}
		if h >= w {
			return ProfileMobilePortrait
		}
		return ProfileMobileLandscape
	}

	if w >= 1400 || (caps.Hover && w >= 1200 && h >= 800) {
		return ProfileDesktopDense
	}
	return ProfileDesktopCompact
}

// SurfaceRole describes how a surface should be treated on mobile and desktop.
type SurfaceRole string

const (
	SurfacePrimary   SurfaceRole = "primary"
	SurfaceSecondary SurfaceRole = "secondary"
	SurfaceOptional  SurfaceRole = "optional"
)

// SurfaceSpec describes one named shell surface in a demo application.
type SurfaceSpec struct {
	ID    string
	Label string
	Role  SurfaceRole
	Notes string
}

// TargetModel defines the expected high-level shell structure for a demo.
type TargetModel struct {
	AppID    string
	AppName  string
	Surfaces []SurfaceSpec
}

// Validate checks that the target model is internally consistent.
func (m TargetModel) Validate() error {
	if m.AppID == "" {
		return fmt.Errorf("target model app id is required")
	}
	if len(m.Surfaces) == 0 {
		return fmt.Errorf("target model must define at least one surface")
	}

	seen := make(map[string]struct{}, len(m.Surfaces))
	hasPrimary := false
	for _, surface := range m.Surfaces {
		if surface.ID == "" {
			return fmt.Errorf("target model has a surface with an empty id")
		}
		if _, ok := seen[surface.ID]; ok {
			return fmt.Errorf("target model has duplicate surface id %q", surface.ID)
		}
		seen[surface.ID] = struct{}{}
		if surface.Role == SurfacePrimary {
			hasPrimary = true
		}
	}

	if !hasPrimary {
		return fmt.Errorf("target model must define at least one primary surface")
	}
	return nil
}

// Surface returns the named surface spec if present.
func (m TargetModel) Surface(id string) (SurfaceSpec, bool) {
	for _, surface := range m.Surfaces {
		if surface.ID == id {
			return surface, true
		}
	}
	return SurfaceSpec{}, false
}

// SurfacesByRole returns all surface specs with the requested role.
func (m TargetModel) SurfacesByRole(role SurfaceRole) []SurfaceSpec {
	if len(m.Surfaces) == 0 {
		return nil
	}
	out := make([]SurfaceSpec, 0, len(m.Surfaces))
	for _, surface := range m.Surfaces {
		if surface.Role == role {
			out = append(out, surface)
		}
	}
	return out
}

// SurfaceIDsByRole returns the IDs of all surfaces with the requested role.
func (m TargetModel) SurfaceIDsByRole(role SurfaceRole) []string {
	surfaces := m.SurfacesByRole(role)
	if len(surfaces) == 0 {
		return nil
	}
	ids := make([]string, 0, len(surfaces))
	for _, surface := range surfaces {
		ids = append(ids, surface.ID)
	}
	sort.Strings(ids)
	return ids
}

// PrimarySurfaceIDs returns the ids of the primary surfaces.
func (m TargetModel) PrimarySurfaceIDs() []string {
	return m.SurfaceIDsByRole(SurfacePrimary)
}

// SecondarySurfaceIDs returns the ids of the secondary surfaces.
func (m TargetModel) SecondarySurfaceIDs() []string {
	return m.SurfaceIDsByRole(SurfaceSecondary)
}

// OptionalSurfaceIDs returns the ids of the optional surfaces.
func (m TargetModel) OptionalSurfaceIDs() []string {
	return m.SurfaceIDsByRole(SurfaceOptional)
}

func clampDimension(v int) int {
	if v < 1 {
		return 1
	}
	return v
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
