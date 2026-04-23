package ui

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
)

// Shell layout constants.
const (
	headerHeight  = 48
	footerHeight  = 32
	sidebarWidth  = 280
	inspectorWidth = 320
	panelGap      = 8
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
func CalculateShellBounds(window gfx.Rect) ShellBounds {
	var s ShellBounds

	s.Header = gfx.RectFromXYWH(window.Min.X, window.Min.Y, window.Width(), headerHeight)
	s.Footer = gfx.RectFromXYWH(window.Min.X, window.Max.Y-footerHeight, window.Width(), footerHeight)

	contentTop := s.Header.Max.Y
	contentBottom := s.Footer.Min.Y
	contentHeight := contentBottom - contentTop

	s.Sidebar = gfx.RectFromXYWH(window.Min.X, contentTop, sidebarWidth, contentHeight)

	inspectorX := window.Max.X - inspectorWidth
	s.Inspector = gfx.RectFromXYWH(inspectorX, contentTop, inspectorWidth, contentHeight)

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
