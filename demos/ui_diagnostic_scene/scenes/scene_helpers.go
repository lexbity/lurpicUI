package scenes

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks/basic"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
)

func newTextMark(id, value string, size float32) *basic.Text {
	return &basic.Text{
		ID: id,
		Paragraph: text.Paragraph{
			Spans: []text.TextSpan{{Text: value, Style: text.TextStyle{Size: size}}},
		},
		MaxWidth:   640,
		Selectable: true,
	}
}

func newActionRect(id string, fill gfx.Color, onClick func()) *basic.Rect {
	rect := &basic.Rect{
		ID: id,
		Bounds: basic.BoundsProps{
			X: 0, Y: 0, W: 120, H: 40,
		},
		Radius: 8,
		Style: basic.PrimitiveStyleProps{
			Fill:    solidFill(fill),
			Stroke:  solidStroke(gfx.ColorFromRGBA8(0, 0, 0, 255), 1),
			Visible: true,
			Opacity: 1,
		},
	}

	base := rect.Base()
	var pressed bool
	base.AddRole(&facet.HitRole{
		OnHitTest: func(p gfx.Point) facet.HitResult {
			if rect.Bounds.Rect().Contains(p) {
				return facet.HitResult{Hit: true, Cursor: facet.CursorPointer}
			}
			return facet.HitResult{}
		},
	})
	base.AddRole(&facet.InputRole{
		OnPointer: func(e facet.PointerEvent) bool {
			switch e.Kind {
			case platform.PointerPress:
				pressed = rect.Bounds.Rect().Contains(e.Position)
				return pressed
			case platform.PointerRelease:
				hit := pressed && rect.Bounds.Rect().Contains(e.Position)
				pressed = false
				if hit && onClick != nil {
					onClick()
				}
				return hit
			default:
				return false
			}
		},
	})
	return rect
}

func tintRectStyle(rect *basic.Rect, fill gfx.Color) {
	if rect == nil {
		return
	}
	rect.Style.Fill = solidFill(fill)
}

func updateTextValue(mark *basic.Text, value string, size float32) {
	if mark == nil {
		return
	}
	mark.Paragraph = text.Paragraph{
		Spans: []text.TextSpan{{Text: value, Style: text.TextStyle{Size: size}}},
	}
}

func themeSampleColors(th theme.Context) []gfx.Color {
	if th == nil {
		th = theme.Default()
	}
	return []gfx.Color{
		th.Color(theme.ColorBackground),
		th.Color(theme.ColorSurface),
		th.Color(theme.ColorPrimary),
		th.Color(theme.ColorSelection),
	}
}
