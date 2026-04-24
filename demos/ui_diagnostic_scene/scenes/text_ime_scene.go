package scenes

import (
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks/basic"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/ui_diagnostic_scene/scene"
)

type TextIMEScene struct {
	BaseScene
	th           theme.Context
	buffer       strings.Builder
	composing    string
	editor       *basic.Rect
	display      *basic.Text
	secondary    *basic.Text
	focused      bool
	densityScale float32
}

func NewTextIMEScene() *TextIMEScene {
	s := &TextIMEScene{
		BaseScene: NewBaseScene(
			"text-ime",
			"Text / IME",
			"Validates text input, composing state, and caret-style updates",
			[]string{"uiinput"},
		),
		th:           theme.Default(),
		densityScale: 1,
	}
	s.capability.HasCustomLogs = true
	return s
}

func (s *TextIMEScene) BuildRoot() facet.FacetImpl {
	if s.root != nil {
		return s.root
	}
	col := layout.NewColumnLayout()
	col.Gap = 10
	s.root = col

	title := newTextMark("ime-title", "Text input and composing state", 18)
	col.AddChild(title.Base())

	s.editor = &basic.Rect{
		ID:     "ime-editor",
		Bounds: basic.BoundsProps{X: 0, Y: 0, W: 420, H: 78},
		Radius: 10,
		Style: basic.PrimitiveStyleProps{
			Fill:    solidFill(s.th.Color(theme.ColorSurface)),
			Stroke:  solidStroke(s.th.Color(theme.ColorBorderStrong), 2),
			Visible: true,
			Opacity: 1,
		},
	}
	s.editor.Base().AddRole(&facet.HitRole{
		OnHitTest: func(p gfx.Point) facet.HitResult {
			if s.editor.Bounds.Rect().Contains(p) {
				return facet.HitResult{Hit: true, Cursor: facet.CursorText}
			}
			return facet.HitResult{}
		},
	})
	s.editor.Base().AddRole(&facet.InputRole{
		OnPointer: func(e facet.PointerEvent) bool {
			if e.Kind == platform.PointerPress && s.editor.Bounds.Rect().Contains(e.Position) {
				s.focused = true
				s.refreshEditor()
				return true
			}
			return false
		},
		OnText: func(e facet.TextEvent) bool {
			if !s.focused || e.Text == "" {
				return false
			}
			if e.Composing {
				s.composing = e.Text
			} else {
				s.buffer.WriteString(e.Text)
				s.composing = ""
			}
			s.refreshEditor()
			return true
		},
		OnKey: func(e facet.KeyEvent) bool {
			if !s.focused || e.Kind != platform.KeyPress {
				return false
			}
			switch e.Key {
			case platform.KeyBackspace:
				txt := s.buffer.String()
				if len(txt) > 0 {
					s.buffer.Reset()
					s.buffer.WriteString(txt[:len(txt)-1])
					s.refreshEditor()
				}
				return true
			case platform.KeyEscape:
				s.focused = false
				s.composing = ""
				s.refreshEditor()
				return true
			default:
				return false
			}
		},
	})
	s.editor.Base().AddRole(&facet.FocusRole{
		Focusable: func() bool { return true },
		OnFocusGained: func() {
			s.focused = true
			s.refreshEditor()
		},
		OnFocusLost: func() {
			s.focused = false
			s.refreshEditor()
		},
	})
	col.AddChild(s.editor.Base())

	s.display = newTextMark("ime-display", "", 14)
	col.AddChild(s.display.Base())

	s.secondary = newTextMark("ime-secondary", "Press the editor, type text, and use Escape or Backspace.", 12)
	col.AddChild(s.secondary.Base())

	s.refreshEditor()
	s.applyDensity()
	return col
}

func (s *TextIMEScene) refreshEditor() {
	textValue := s.buffer.String()
	if s.composing != "" {
		textValue += " [" + s.composing + "]"
	}
	updateTextValue(s.display, textValue, 14)
	if s.editor != nil {
		fill := s.th.Color(theme.ColorSurface)
		if s.focused {
			fill = s.th.Color(theme.ColorSelection)
		}
		tintRectStyle(s.editor, fill)
	}
}

func (s *TextIMEScene) applyDensity() {
	if s.editor == nil {
		return
	}
	s.editor.Bounds.W = 420 * s.densityScale
	s.editor.Bounds.H = 78 * s.densityScale
}

func (s *TextIMEScene) ApplyTheme(th theme.Context) {
	s.th = th
	s.refreshEditor()
	if s.root != nil && s.root.Base() != nil {
		s.root.Base().Invalidate(facet.DirtyProjection)
	}
}

func (s *TextIMEScene) ApplyDensity(scale float32) {
	if scale <= 0 {
		scale = 1
	}
	s.densityScale = scale
	s.applyDensity()
	if s.root != nil && s.root.Base() != nil {
		s.root.Base().Invalidate(facet.DirtyLayout | facet.DirtyProjection)
	}
}

func (s *TextIMEScene) Reset() {
	s.buffer.Reset()
	s.composing = ""
	s.focused = false
	s.refreshEditor()
	s.BaseScene.Reset()
}

func (s *TextIMEScene) ExportState() map[string]any {
	return map[string]any{
		"scene_id":  s.id,
		"text":      s.buffer.String(),
		"composing": s.composing,
		"density":   s.densityScale,
	}
}

func (s *TextIMEScene) ImportState(state map[string]any) {
	if v, ok := state["text"].(string); ok {
		s.buffer.Reset()
		s.buffer.WriteString(v)
	}
	if v, ok := state["composing"].(string); ok {
		s.composing = v
	}
	if v, ok := state["density"].(float64); ok {
		s.densityScale = float32(v)
	}
	s.refreshEditor()
}

var _ scene.Scene = (*TextIMEScene)(nil)
