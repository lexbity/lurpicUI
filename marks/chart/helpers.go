package chart

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
