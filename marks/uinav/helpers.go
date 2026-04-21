package uinav

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
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

func syncLayout(layoutRole *facet.LayoutRole, bounds gfx.Rect) {
	if layoutRole == nil {
		return
	}
	layoutRole.Arrange(bounds)
	layoutRole.MeasuredSize = gfx.Size{W: bounds.Width(), H: bounds.Height()}
}

func syncViewport(viewport *facet.ViewportRole, transform gfx.Transform) {
	if viewport == nil {
		return
	}
	viewport.Transform = transform
}

func fillColor(m theme.Material, fallback gfx.Color) gfx.Color {
	for _, fill := range m.Fills {
		if fill.Type == theme.FillSolid || fill.Type == theme.FillNone {
			return fill.Color.WithAlpha(fill.Color.A * fill.Opacity * m.Opacity)
		}
	}
	return fallback
}

func strokeColor(m theme.Material, fallback gfx.Color) gfx.Color {
	for _, stroke := range m.Strokes {
		if stroke.Paint.Type == theme.FillSolid || stroke.Paint.Type == theme.FillNone {
			return stroke.Paint.Color.WithAlpha(stroke.Paint.Color.A * stroke.Paint.Opacity * m.Opacity)
		}
	}
	return fallback
}

func strokeStyle(stroke theme.MaterialStroke) gfx.StrokeStyle {
	style := gfx.DefaultStroke(stroke.Width)
	style.Dash = append([]float32(nil), stroke.Dash...)
	style.DashOffset = stroke.DashOffset
	return style
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
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
