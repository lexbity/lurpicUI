package ui

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/ui_catalog/store"
)

// Shell layout constants.
const (
	// Density scales are applied around the normal profile.
	headerHeightNormal          = 48
	footerHeightNormal          = 32
	sidebarWidthMinNormal       = 200
	sidebarWidthMaxNormal       = 320
	sidebarWidthDefaultNormal   = 240
	inspectorWidthMinNormal     = 200
	inspectorWidthMaxNormal     = 360
	inspectorWidthDefaultNormal = 280
	contentPaddingNormal        = 16
	panelGapNormal              = 8
	cardWidthNormal             = 160
	cardHeightNormal            = 100
	cardMarginNormal            = 12
	headerInsetNormal           = 16
	footerInsetNormal           = 8
	sidebarInsetNormal          = 12
	inspectorInsetNormal        = 12
	fieldGapNormal              = 4
	fieldLabelWidthNormal       = 80
	contentFamilyHeaderNormal   = 24
	contentComparePadNormal     = 8
	contentCompareGapNormal     = 16
)

// LayoutProfile captures density-adjusted geometry for the catalog shell.
type LayoutProfile struct {
	Density store.DensityMode
	Scale   float32

	HeaderHeight float32
	FooterHeight float32

	SidebarWidthMin     float32
	SidebarWidthMax     float32
	SidebarWidthDefault float32

	InspectorWidthMin     float32
	InspectorWidthMax     float32
	InspectorWidthDefault float32

	ContentPadding float32
	PanelGap       float32

	CardWidth  float32
	CardHeight float32
	CardMargin float32

	HeaderInset        float32
	FooterInset        float32
	SidebarInset       float32
	InspectorInset     float32
	FieldGap           float32
	FieldLabelWidth    float32
	FamilyHeaderHeight float32
	CompareInnerPad    float32
	ComparePanelGap    float32
}

func LayoutProfileForDensity(mode store.DensityMode) LayoutProfile {
	scale := mode.SpacingScale()
	if scale <= 0 {
		scale = 1
	}

	return LayoutProfile{
		Density: mode,
		Scale:   scale,

		HeaderHeight: headerHeightNormal * scale,
		FooterHeight: footerHeightNormal * scale,

		SidebarWidthMin:     sidebarWidthMinNormal * scale,
		SidebarWidthMax:     sidebarWidthMaxNormal * scale,
		SidebarWidthDefault: sidebarWidthDefaultNormal * scale,

		InspectorWidthMin:     inspectorWidthMinNormal * scale,
		InspectorWidthMax:     inspectorWidthMaxNormal * scale,
		InspectorWidthDefault: inspectorWidthDefaultNormal * scale,

		ContentPadding: contentPaddingNormal * scale,
		PanelGap:       panelGapNormal * scale,

		CardWidth:  cardWidthNormal * scale,
		CardHeight: cardHeightNormal * scale,
		CardMargin: cardMarginNormal * scale,

		HeaderInset:        headerInsetNormal * scale,
		FooterInset:        footerInsetNormal * scale,
		SidebarInset:       sidebarInsetNormal * scale,
		InspectorInset:     inspectorInsetNormal * scale,
		FieldGap:           fieldGapNormal * scale,
		FieldLabelWidth:    fieldLabelWidthNormal * scale,
		FamilyHeaderHeight: contentFamilyHeaderNormal * scale,
		CompareInnerPad:    contentComparePadNormal * scale,
		ComparePanelGap:    contentCompareGapNormal * scale,
	}
}

// DefaultLayoutProfile returns the profile for the current density selection.
func DefaultLayoutProfile() LayoutProfile {
	return LayoutProfileForDensity(store.GetDensity())
}

// ShellBounds holds computed bounds for all shell regions.
type ShellBounds struct {
	Header    gfx.Rect
	Sidebar   gfx.Rect
	Content   gfx.Rect
	Inspector gfx.Rect
	Footer    gfx.Rect
}

// CalculateShellBounds computes layout regions from window bounds.
func CalculateShellBounds(window gfx.Rect, sidebarWidth, inspectorWidth float32) ShellBounds {
	return CalculateShellBoundsWithProfile(window, sidebarWidth, inspectorWidth, DefaultLayoutProfile())
}

// CalculateShellBoundsWithProfile computes layout regions from window bounds.
func CalculateShellBoundsWithProfile(window gfx.Rect, sidebarWidth, inspectorWidth float32, profile LayoutProfile) ShellBounds {
	var s ShellBounds

	// Header at top
	s.Header = gfx.RectFromXYWH(window.Min.X, window.Min.Y, window.Width(), profile.HeaderHeight)

	// Footer at bottom
	s.Footer = gfx.RectFromXYWH(window.Min.X, window.Max.Y-profile.FooterHeight, window.Width(), profile.FooterHeight)

	// Available vertical space
	contentTop := s.Header.Max.Y
	contentBottom := s.Footer.Min.Y
	contentHeight := contentBottom - contentTop

	// Sidebar on left
	if sidebarWidth < profile.SidebarWidthMin {
		sidebarWidth = profile.SidebarWidthMin
	}
	if sidebarWidth > profile.SidebarWidthMax {
		sidebarWidth = profile.SidebarWidthMax
	}
	s.Sidebar = gfx.RectFromXYWH(window.Min.X, contentTop, sidebarWidth, contentHeight)

	// Inspector on right
	if inspectorWidth < profile.InspectorWidthMin {
		inspectorWidth = profile.InspectorWidthMin
	}
	if inspectorWidth > profile.InspectorWidthMax {
		inspectorWidth = profile.InspectorWidthMax
	}
	inspectorX := window.Max.X - inspectorWidth
	s.Inspector = gfx.RectFromXYWH(inspectorX, contentTop, inspectorWidth, contentHeight)

	// Content in middle
	contentX := s.Sidebar.Max.X + profile.PanelGap
	contentWidth := s.Inspector.Min.X - contentX - profile.PanelGap
	if contentWidth < 0 {
		contentWidth = 0
	}
	s.Content = gfx.RectFromXYWH(contentX, contentTop, contentWidth, contentHeight)

	return s
}

// Inset returns bounds inset by padding if space permits.
func Inset(bounds gfx.Rect, pad float32) gfx.Rect {
	if bounds.IsEmpty() {
		return gfx.Rect{}
	}
	if bounds.Width() <= pad*2 || bounds.Height() <= pad*2 {
		return gfx.Rect{Min: bounds.Min, Max: bounds.Min}
	}
	return bounds.Inset(pad, pad)
}

type facetChildAdder interface {
	AddFacet(parent, child facet.FacetImpl, attachment layout.ChildAttachment)
}

func attachChild(parent *facet.Facet, parentImpl facet.FacetImpl, child facet.FacetImpl, adder facetChildAdder, attachment layout.ChildAttachment) {
	if parent == nil || child == nil {
		return
	}
	if adder != nil {
		adder.AddFacet(parentImpl, child, attachment)
		return
	}
	parent.AddChildRuntime(child.Base())
}
