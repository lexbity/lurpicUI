package uinav

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/internal/markutil"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
)

type controlState struct {
	hovered  bool
	pressed  bool
	focused  bool
	disabled bool
}

func registerDescriptor(d marks.Descriptor) {
	marks.RegisterDescriptor(d)
}

func ensureBase(base *facet.Facet) {
	if base == nil {
		return
	}
	if base.ID() == 0 {
		*base = facet.NewFacet()
	}
}

func invalidate(base *facet.Facet, flags facet.DirtyFlags, source string) {
	if base == nil {
		return
	}
	base.InvalidateWithSource(flags, source)
}

func syncLayout(layoutRole *facet.LayoutRole, bounds gfx.Rect) {
	markutil.SyncLayout(layoutRole, bounds)
}

func syncViewport(viewport *facet.ViewportRole, transform gfx.Transform) {
	markutil.SyncViewport(viewport, transform)
}

func fillColor(m theme.Material, fallback gfx.Color) gfx.Color {
	return markutil.FillColor(m, fallback)
}

func strokeColor(m theme.Material, fallback gfx.Color) gfx.Color {
	return markutil.StrokeColor(m, fallback)
}

func strokeStyle(stroke theme.MaterialStroke) gfx.StrokeStyle {
	return markutil.StrokeStyle(stroke)
}

func drawText(list *gfx.CommandList, shaper *text.Shaper, x, y float32, s string, style text.TextStyle, color gfx.Color) {
	markutil.DrawText(list, shaper, x, y, s, style, color)
}

func clampInt(v, min, max int) int {
	return markutil.ClampInt(v, min, max)
}

func clampFloat(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func tabsHeight() float32 {
	return markutil.RegularUINavBaseline().Tabs.Height.Regular
}

func tabsIndicatorThickness() float32 {
	return markutil.RegularUINavBaseline().Tabs.IndicatorThickness.Regular
}

func drawerMinWidth() float32 {
	return markutil.RegularUINavBaseline().Drawer.MinWidth.Regular - menuPadding()*2
}

func drawerMaxWidth() float32 {
	return markutil.RegularUINavBaseline().Drawer.MinWidth.Regular + menuPadding()*8
}

func menuRowHeight() float32 {
	return markutil.RegularUINavBaseline().Menu.RowHeight.Regular - 8
}

func menuPadding() float32 {
	return markutil.RegularUINavBaseline().Menu.Padding.Regular
}

func scrollbarThickness() float32 {
	return markutil.RegularUINavBaseline().Scrollbar.Thickness.Regular + 2
}

func paginationItemSize() float32 {
	return markutil.RegularUINavBaseline().Pagination.ItemSize.Regular
}

func rectFromSize(w, h float32) gfx.Rect {
	return gfx.RectFromXYWH(0, 0, w, h)
}

func attachChildMarks(parent *facet.Facet, children []marks.Mark) {
	if parent == nil {
		return
	}
	for _, child := range children {
		if child == nil {
			continue
		}
		impl, ok := child.(facet.FacetImpl)
		if !ok {
			panic("marks/uinav: child mark does not implement facet.FacetImpl")
		}
		parent.AddChild(impl.Base())
	}
}

func attachSingleChild(parent *facet.Facet, child marks.Mark) {
	if child == nil {
		return
	}
	attachChildMarks(parent, []marks.Mark{child})
}

func transformAnchors(tx gfx.Transform, anchors layout.AnchorSet) layout.AnchorSet {
	if len(anchors) == 0 {
		return nil
	}
	out := make(layout.AnchorSet, len(anchors))
	for id, pt := range anchors {
		out[id] = tx.TransformPoint(pt)
	}
	return out
}

func boundsAnchors(bounds gfx.Rect) layout.AnchorSet {
	if bounds.IsEmpty() {
		return nil
	}
	return layout.AnchorSet{
		"bounds-center": {X: bounds.Min.X + bounds.Width()/2, Y: bounds.Min.Y + bounds.Height()/2},
		"top-left":      {X: bounds.Min.X, Y: bounds.Min.Y},
		"top-right":     {X: bounds.Max.X, Y: bounds.Min.Y},
		"bottom-right":  {X: bounds.Max.X, Y: bounds.Max.Y},
		"bottom-left":   {X: bounds.Min.X, Y: bounds.Max.Y},
	}
}

func findMarkedChild(base *facet.Facet, markID string) facet.FacetImpl {
	if base == nil || markID == "" {
		return nil
	}
	stack := []*facet.Facet{base}
	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if current == nil {
			continue
		}
		impl := current.Impl()
		if impl != nil {
			if authored, ok := impl.(interface{ AuthoredID() string }); ok && authored.AuthoredID() == markID || fmt.Sprint(current.ID()) == markID {
				return impl
			}
		}
		children := current.Children()
		for i := len(children) - 1; i >= 0; i-- {
			stack = append(stack, children[i])
		}
	}
	return nil
}

func rootFacet(base *facet.Facet) *facet.Facet {
	if base == nil {
		return nil
	}
	current := base
	for parent := current.Parent(); parent != nil; parent = current.Parent() {
		current = parent
	}
	return current
}

func anchorPoint(root *facet.Facet, ref AnchorSourceRef, fallback layout.AnchorID) (gfx.Point, bool) {
	if root == nil {
		return gfx.Point{}, false
	}
	target := findMarkedChild(root, ref.MarkID)
	if target == nil {
		return gfx.Point{}, false
	}
	exporter, ok := target.(layout.AnchorExporter)
	if !ok {
		return gfx.Point{}, false
	}
	anchors := exporter.ExportAnchors(layout.AnchorExportContext{})
	if len(anchors) == 0 {
		return gfx.Point{}, false
	}
	if ref.Anchor != "" {
		if pt, ok := anchors[layout.AnchorID(ref.Anchor)]; ok {
			return pt, true
		}
		return gfx.Point{}, false
	}
	if pt, ok := anchors[fallback]; ok {
		return pt, true
	}
	if pt, ok := anchors["bounds-center"]; ok {
		return pt, true
	}
	for _, pt := range anchors {
		return pt, true
	}
	return gfx.Point{}, false
}
