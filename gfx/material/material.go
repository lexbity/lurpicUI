package material

import (
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/theme"
)

// Commands converts a theme material into drawable commands for a path.
func Commands(path gfx.Path, material theme.Material) []gfx.Command {
	if theme.Transparent(material) {
		return nil
	}

	cmds := make([]gfx.Command, 0, len(material.Fills)+len(material.Strokes))
	materialOpacity := clampOpacity(material.Opacity)
	for _, fill := range material.Fills {
		switch fill.Type {
		case theme.FillSolid:
			if fill.Color.A <= 0 || fill.Opacity <= 0 {
				continue
			}
			cmds = append(cmds, gfx.FillPath{
				Path:  path,
				Brush: gfx.SolidBrush(scaleColor(fill.Color, materialOpacity*fill.Opacity)),
			})
		case theme.FillGradient:
			if fill.Opacity <= 0 || fill.Gradient.Type != theme.GradientLinear || len(fill.Gradient.Stops) == 0 {
				continue
			}
			stops := make([]gfx.GradientStop, len(fill.Gradient.Stops))
			for i, stop := range fill.Gradient.Stops {
				stops[i] = gfx.GradientStop{Offset: stop.Position, Color: scaleColor(stop.Color, materialOpacity*fill.Opacity)}
			}
			cmds = append(cmds, gfx.FillPath{
				Path:  path,
				Brush: gfx.LinearGradientBrush(fill.Gradient.Start, fill.Gradient.End, stops),
			})
		}
	}
	for _, stroke := range material.Strokes {
		if stroke.Width <= 0 || stroke.Paint.Type != theme.FillSolid || stroke.Paint.Color.A <= 0 || stroke.Paint.Opacity <= 0 {
			continue
		}
		cmds = append(cmds, gfx.StrokePath{
			Path:  path,
			Brush: gfx.SolidBrush(scaleColor(stroke.Paint.Color, materialOpacity*stroke.Paint.Opacity)),
			Stroke: gfx.StrokeStyle{
				Width:      stroke.Width,
				Cap:        convertLineCap(stroke.Cap),
				Join:       convertLineJoin(stroke.Join),
				MiterLimit: 10,
				Dash:       append([]float32(nil), stroke.Dash...),
				DashOffset: stroke.DashOffset,
			},
		})
	}
	return cmds
}

func scaleColor(c gfx.Color, opacity float32) gfx.Color {
	opacity = clampOpacity(opacity)
	if opacity <= 0 {
		return gfx.Color{}
	}
	if opacity >= 1 {
		return c
	}
	return gfx.Color{R: c.R * opacity, G: c.G * opacity, B: c.B * opacity, A: c.A * opacity}
}

func clampOpacity(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func convertLineCap(cap theme.StrokeCap) gfx.LineCap {
	switch cap {
	case theme.CapRound:
		return gfx.LineCapRound
	case theme.CapSquare:
		return gfx.LineCapSquare
	default:
		return gfx.LineCapButt
	}
}

func convertLineJoin(join theme.StrokeJoin) gfx.LineJoin {
	switch join {
	case theme.JoinRound:
		return gfx.LineJoinRound
	case theme.JoinBevel:
		return gfx.LineJoinBevel
	default:
		return gfx.LineJoinMiter
	}
}
