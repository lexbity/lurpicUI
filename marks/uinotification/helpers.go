package uinotification

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/theme"
)

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
			panic("marks/uinotification: child mark does not implement facet.FacetImpl")
		}
		parent.AddChild(impl.Base())
	}
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
