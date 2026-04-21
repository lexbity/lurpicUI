package uiinput

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
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
	switch stroke.Cap {
	case theme.CapRound:
		style.Cap = gfx.LineCapRound
	case theme.CapSquare:
		style.Cap = gfx.LineCapSquare
	default:
		style.Cap = gfx.LineCapButt
	}
	switch stroke.Join {
	case theme.JoinRound:
		style.Join = gfx.LineJoinRound
	case theme.JoinBevel:
		style.Join = gfx.LineJoinBevel
	default:
		style.Join = gfx.LineJoinMiter
	}
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
