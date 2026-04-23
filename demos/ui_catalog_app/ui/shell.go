package ui

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
)

// Shell layout constants.
const (
	// Header
	headerHeight = 48

	// Footer
	footerHeight = 32

	// Sidebar
	sidebarWidthMin     = 200
	sidebarWidthMax     = 320
	sidebarWidthDefault = 240

	// Inspector (right panel)
	inspectorWidthMin     = 200
	inspectorWidthMax     = 360
	inspectorWidthDefault = 280

	// Content area padding
	contentPadding = 16

	// Panel gaps
	panelGap = 8
)

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
	var s ShellBounds

	// Header at top
	s.Header = gfx.RectFromXYWH(window.Min.X, window.Min.Y, window.Width(), headerHeight)

	// Footer at bottom
	s.Footer = gfx.RectFromXYWH(window.Min.X, window.Max.Y-footerHeight, window.Width(), footerHeight)

	// Available vertical space
	contentTop := s.Header.Max.Y
	contentBottom := s.Footer.Min.Y
	contentHeight := contentBottom - contentTop

	// Sidebar on left
	if sidebarWidth < sidebarWidthMin {
		sidebarWidth = sidebarWidthMin
	}
	if sidebarWidth > sidebarWidthMax {
		sidebarWidth = sidebarWidthMax
	}
	s.Sidebar = gfx.RectFromXYWH(window.Min.X, contentTop, sidebarWidth, contentHeight)

	// Inspector on right
	if inspectorWidth < inspectorWidthMin {
		inspectorWidth = inspectorWidthMin
	}
	if inspectorWidth > inspectorWidthMax {
		inspectorWidth = inspectorWidthMax
	}
	inspectorX := window.Max.X - inspectorWidth
	s.Inspector = gfx.RectFromXYWH(inspectorX, contentTop, inspectorWidth, contentHeight)

	// Content in middle
	contentX := s.Sidebar.Max.X + panelGap
	contentWidth := s.Inspector.Min.X - contentX - panelGap
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
