package markutil

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/lurpicui/theme/baseline"
)

// SyncLayout applies a measured bounds rectangle to a layout role.
func SyncLayout(layoutRole *facet.LayoutRole, bounds gfx.Rect) {
	if layoutRole == nil {
		return
	}
	layoutRole.Arrange(bounds)
	layoutRole.MeasuredSize = gfx.Size{W: bounds.Width(), H: bounds.Height()}
}

// SyncViewport applies an authored local transform to a viewport role.
func SyncViewport(viewport *facet.ViewportRole, transform gfx.Transform) {
	if viewport == nil {
		return
	}
	viewport.Transform = transform
}

// FillColor extracts a solid fill color from a material.
func FillColor(m theme.Material, fallback gfx.Color) gfx.Color {
	for _, fill := range m.Fills {
		if fill.Type == theme.FillSolid || fill.Type == theme.FillNone {
			return fill.Color.WithAlpha(fill.Color.A * fill.Opacity * m.Opacity)
		}
	}
	return fallback
}

// StrokeColor extracts a solid stroke color from a material.
func StrokeColor(m theme.Material, fallback gfx.Color) gfx.Color {
	for _, stroke := range m.Strokes {
		if stroke.Paint.Type == theme.FillSolid || stroke.Paint.Type == theme.FillNone {
			return stroke.Paint.Color.WithAlpha(stroke.Paint.Color.A * stroke.Paint.Opacity * m.Opacity)
		}
	}
	return fallback
}

// StrokeStyle converts a material stroke into a gfx stroke style.
func StrokeStyle(stroke theme.MaterialStroke) gfx.StrokeStyle {
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

// DrawText draws a shaped text run at the supplied origin.
func DrawText(list *gfx.CommandList, shaper *text.Shaper, x, y float32, s string, style text.TextStyle, color gfx.Color) {
	if list == nil || shaper == nil || s == "" {
		return
	}
	layout := shaper.ShapeSimple(s, style)
	if layout == nil || len(layout.Lines) == 0 {
		return
	}
	for _, line := range layout.Lines {
		origin := gfx.Point{X: x + line.Bounds.Min.X, Y: y + line.Baseline}
		for _, run := range line.Runs {
			list.Add(gfx.DrawGlyphRun{
				Run:    run,
				Origin: origin,
				Brush:  gfx.SolidBrush(color),
			})
		}
	}
}

// ClampInt bounds v to [min, max].
func ClampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// RegularUIInputBaseline returns the default regular-density input baseline.
func RegularUIInputBaseline() baseline.UIInputBaseline {
	return baseline.Default().UIInput
}

// RegularUINavBaseline returns the default regular-density navigation baseline.
func RegularUINavBaseline() baseline.UINavBaseline {
	return baseline.Default().UINav
}

// RegularUINotificationBaseline returns the default regular-density notification baseline.
func RegularUINotificationBaseline() baseline.UINotificationBaseline {
	return baseline.Default().UINotification
}
