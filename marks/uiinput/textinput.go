package uiinput

import (
	"sync"
	"unicode/utf8"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
)

// TextInput is a simple editable text control.
type TextInput struct {
	ID          string
	Value       store.Binding[string]
	Variant     TextInputVariant
	Placeholder string
	Disabled    bool
	ReadOnly    bool
	Multiline   bool
	Assistive   string
	Theme       theme.Context
	Shaper      *text.Shaper

	base         facet.Facet
	once         sync.Once
	state        controlState
	cursor       int
	selection    text.TextRange
	composing    string
	dragAnchor   int
	dragging     bool
	layoutRole   *facet.LayoutRole
	viewportRole *facet.ViewportRole
	projection   *facet.ProjectionRole
	hitRole      *facet.HitRole
	inputRole    *facet.InputRole
	focusRole    *facet.FocusRole
}

func init() {
	registerDescriptor(marks.Descriptor{
		Family:            marks.FamilyUIInput,
		ConstructionClass: marks.ConstructionComposed,
		Type:              marks.TypeName("uiinput:textinput"),
		Focusable:         true,
		HitTestable:       true,
	})
}

func (t *TextInput) Base() *facet.Facet { t.ensureInit(); return &t.base }
func (t *TextInput) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: marks.FamilyUIInput, ConstructionClass: marks.ConstructionComposed, Type: marks.TypeName("uiinput:textinput"), Focusable: true, HitTestable: true}
}
func (t *TextInput) AuthoredID() string { return t.ID }
func (t *TextInput) OnAttach(ctx facet.AttachContext) {
	t.syncRoles()
	s := facet.Subscribe(t)
	if st := t.Value.Store(); st != nil {
		facet.To(s, &st.OnChange, func(signal.Change[string]) {
			t.rebuildLayout()
			invalidate(&t.base, facet.DirtyLayout|facet.DirtyProjection, "text-input-bind")
		})
	}
}
func (t *TextInput) OnDetach()     {}
func (t *TextInput) OnActivate()   {}
func (t *TextInput) OnDeactivate() {}

func (t *TextInput) ensureInit() {
	t.once.Do(func() {
		ensureBase(&t.base)
		t.base.BindImpl(t)
		t.layoutRole = &facet.LayoutRole{OnMeasure: func(c facet.Constraints) gfx.Size {
			bounds := t.bounds()
			return gfx.Size{W: bounds.Width(), H: bounds.Height()}
		}}
		t.viewportRole = &facet.ViewportRole{Transform: gfx.Identity()}
		t.projection = &facet.ProjectionRole{OnProject: func(ctx facet.ProjectionContext) *gfx.CommandList { return t.project(ctx) }}
		t.hitRole = &facet.HitRole{OnHitTest: func(p gfx.Point) facet.HitResult {
			if t.bounds().Contains(p) {
				return facet.HitResult{Hit: true, Cursor: facet.CursorText}
			}
			return facet.HitResult{}
		}}
		t.inputRole = &facet.InputRole{
			OnPointer: func(e facet.PointerEvent) bool { return t.handlePointer(e) },
			OnKey:     func(e facet.KeyEvent) bool { return t.handleKey(e) },
			OnText:    func(e facet.TextEvent) bool { return t.handleText(e) },
		}
		t.focusRole = &facet.FocusRole{
			Focusable:     func() bool { return !t.Disabled },
			OnFocusGained: func() { t.state.focused = true },
			OnFocusLost: func() {
				t.state.focused = false
				t.dragging = false
			},
		}
		t.base.AddRole(t.layoutRole)
		t.base.AddRole(t.viewportRole)
		t.base.AddRole(t.projection)
		t.base.AddRole(t.hitRole)
		t.base.AddRole(t.inputRole)
		t.base.AddRole(t.focusRole)
		t.syncRoles()
	})
}

func (t *TextInput) syncRoles() {
	t.state.disabled = t.Disabled
	t.rebuildLayout()
}

func (t *TextInput) bounds() gfx.Rect {
	if t.layoutRole != nil && !t.layoutRole.ArrangedBounds.IsEmpty() {
		return t.layoutRole.ArrangedBounds
	}
	layout := t.currentLayout()
	if layout == nil || layout.Bounds.IsEmpty() {
		if t.Multiline {
			return gfx.RectFromXYWH(0, 0, textInputMinWidth()*2, textInputMultilineHeight())
		}
		return gfx.RectFromXYWH(0, 0, textInputMinWidth()*2, buttonHeight())
	}
	height := layout.Bounds.Height()
	if height < 20 {
		height = 20
	}
	padY := textInputPaddingY()
	return gfx.RectFromXYWH(0, 0, maxf(textInputMinWidth()*2, layout.Bounds.Width()+16), height+padY*2)
}

func (t *TextInput) currentText() string {
	if t.composing != "" {
		return t.Value.Get() + t.composing
	}
	return t.Value.Get()
}

func (t *TextInput) currentLayout() *text.TextLayout {
	return simpleTextLayout(t.currentText(), t.Multiline)
}

func (t *TextInput) rebuildLayout() {
	layout := t.currentLayout()
	if layout == nil {
		t.cursor = 0
		t.selection = text.TextRange{}
		return
	}
	runeCount := layout.RuneCount()
	if t.cursor > runeCount {
		t.cursor = runeCount
	}
	if t.selection.Start < 0 {
		t.selection.Start = 0
	}
	if t.selection.End < 0 {
		t.selection.End = 0
	}
}

func (t *TextInput) handlePointer(e facet.PointerEvent) bool {
	if t.Disabled {
		return false
	}
	bounds := t.bounds()
	layout := t.currentLayout()
	if layout == nil {
		layout = simpleTextLayout("", t.Multiline)
	}
	local := text.Point{X: e.Position.X - bounds.Min.X, Y: e.Position.Y - bounds.Min.Y}
	switch e.Kind {
	case platform.PointerPress:
		t.state.focused = true
		pos := layout.HitTest(text.Point{X: local.X - 8, Y: local.Y - 8})
		t.cursor = clampInt(pos.Index, 0, layout.RuneCount())
		t.selection = text.TextRange{Start: t.cursor, End: t.cursor}
		t.dragAnchor = t.cursor
		t.dragging = true
		invalidate(&t.base, facet.DirtyProjection, "text-input-press")
		return true
	case platform.PointerMove:
		if t.dragging {
			pos := layout.HitTest(text.Point{X: local.X - 8, Y: local.Y - 8})
			t.cursor = clampInt(pos.Index, 0, layout.RuneCount())
			t.selection = text.TextRange{Start: t.dragAnchor, End: t.cursor}
			invalidate(&t.base, facet.DirtyProjection, "text-input-drag")
			return true
		}
	case platform.PointerRelease:
		if t.dragging {
			pos := layout.HitTest(text.Point{X: local.X - 8, Y: local.Y - 8})
			t.cursor = clampInt(pos.Index, 0, layout.RuneCount())
			t.selection = text.TextRange{Start: t.dragAnchor, End: t.cursor}
			t.dragging = false
			invalidate(&t.base, facet.DirtyProjection, "text-input-release")
			return true
		}
	}
	return false
}

func (t *TextInput) handleKey(e facet.KeyEvent) bool {
	if t.Disabled || !t.state.focused || e.Kind != platform.KeyPress {
		return false
	}
	textValue := t.Value.Get()
	runes := []rune(textValue)
	cursor := clampInt(t.cursor, 0, len(runes))
	sel := t.selection.Normalized()
	if sel.End > len(runes) {
		sel.End = len(runes)
	}
	if sel.Start > len(runes) {
		sel.Start = len(runes)
	}
	applyInsert := func(insert string) {
		textValue = replaceRunes(textValue, sel.Start, sel.End, insert)
		t.Value.Set(textValue)
		t.cursor = sel.Start + utf8.RuneCountInString(insert)
		t.selection = text.TextRange{Start: t.cursor, End: t.cursor}
		t.rebuildLayout()
	}
	switch e.Key {
	case platform.KeyLeft:
		if e.Modifiers&platform.ModShift != 0 {
			if t.selection.IsEmpty() {
				t.selection = text.TextRange{Start: cursor, End: cursor}
			}
			if cursor > 0 {
				cursor--
			}
			t.cursor = cursor
			t.selection.End = cursor
		} else {
			if cursor > 0 {
				cursor--
			}
			t.cursor = cursor
			t.selection = text.TextRange{Start: cursor, End: cursor}
		}
		invalidate(&t.base, facet.DirtyProjection, "text-input-key-left")
		return true
	case platform.KeyRight:
		if e.Modifiers&platform.ModShift != 0 {
			if t.selection.IsEmpty() {
				t.selection = text.TextRange{Start: cursor, End: cursor}
			}
			if cursor < len(runes) {
				cursor++
			}
			t.cursor = cursor
			t.selection.End = cursor
		} else {
			if cursor < len(runes) {
				cursor++
			}
			t.cursor = cursor
			t.selection = text.TextRange{Start: cursor, End: cursor}
		}
		invalidate(&t.base, facet.DirtyProjection, "text-input-key-right")
		return true
	case platform.KeyHome:
		t.cursor = 0
		if e.Modifiers&platform.ModShift != 0 {
			t.selection.End = 0
		} else {
			t.selection = text.TextRange{}
		}
		invalidate(&t.base, facet.DirtyProjection, "text-input-key-home")
		return true
	case platform.KeyEnd:
		t.cursor = len(runes)
		if e.Modifiers&platform.ModShift != 0 {
			t.selection.End = len(runes)
		} else {
			t.selection = text.TextRange{}
		}
		invalidate(&t.base, facet.DirtyProjection, "text-input-key-end")
		return true
	case platform.KeyBackspace:
		if t.ReadOnly {
			return true
		}
		if !sel.IsEmpty() {
			applyInsert("")
			invalidate(&t.base, facet.DirtyLayout|facet.DirtyProjection, "text-input-backspace")
			return true
		}
		if cursor > 0 {
			textValue = replaceRunes(textValue, cursor-1, cursor, "")
			t.Value.Set(textValue)
			t.cursor = cursor - 1
			t.selection = text.TextRange{Start: t.cursor, End: t.cursor}
			t.rebuildLayout()
			invalidate(&t.base, facet.DirtyLayout|facet.DirtyProjection, "text-input-backspace")
		}
		return true
	case platform.KeyEnter:
		if t.Multiline && !t.ReadOnly {
			applyInsert("\n")
			invalidate(&t.base, facet.DirtyLayout|facet.DirtyProjection, "text-input-enter")
		}
		return true
	default:
		return false
	}
}

func (t *TextInput) handleText(e facet.TextEvent) bool {
	if t.Disabled || t.ReadOnly {
		return false
	}
	if e.Composing {
		t.composing = e.Text
		invalidate(&t.base, facet.DirtyProjection, "text-input-compose")
		return true
	}
	selection := t.selection.Normalized()
	textValue := replaceRunes(t.Value.Get(), selection.Start, selection.End, e.Text)
	t.Value.Set(textValue)
	t.cursor = selection.Start + utf8.RuneCountInString(e.Text)
	t.selection = text.TextRange{Start: t.cursor, End: t.cursor}
	t.composing = ""
	t.rebuildLayout()
	invalidate(&t.base, facet.DirtyLayout|facet.DirtyProjection, "text-input-text")
	return true
}

func (t *TextInput) project(ctx facet.ProjectionContext) *gfx.CommandList {
	_ = ctx
	th := t.themeContext()
	layout := t.currentLayout()
	var list gfx.CommandList
	bounds := t.bounds()
	field := th.Color(theme.ColorSurface)
	outline := th.Color(theme.ColorBorder)
	if field.A == 0 {
		field = gfx.Color{R: 0.14, G: 0.15, B: 0.18, A: 1}
	}
	list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(field)})
	list.Add(gfx.StrokeRect{Rect: bounds, Brush: gfx.SolidBrush(outline)})
	if t.state.focused {
		list.Add(gfx.StrokeRect{Rect: bounds.Inset(-2, -2), Brush: gfx.SolidBrush(th.Color(theme.ColorPrimary))})
	}
	if layout != nil {
		origin := gfx.Point{X: bounds.Min.X + 12, Y: bounds.Min.Y + 10}
		if t.selection.IsEmpty() {
			if t.state.focused {
				caretRect := textRectToGfx(layout.CaretRect(text.TextPosition{Index: t.cursor})).Offset(origin.X, origin.Y)
				list.Add(gfx.FillRect{Rect: caretRect, Brush: gfx.SolidBrush(th.Color(theme.ColorCaret))})
			}
		} else {
			for _, rect := range layout.SelectionRects(t.selection) {
				list.Add(gfx.DrawSelectionRects{Rects: []gfx.Rect{textRectToGfx(rect).Offset(origin.X, origin.Y)}, Brush: gfx.SolidBrush(th.Color(theme.ColorSelection))})
			}
		}
	}
	if t.Value.Get() == "" && t.Placeholder != "" {
		drawText(&list, t.Shaper, bounds.Min.X+12, bounds.Min.Y+8, t.Placeholder, th.TextStyle(theme.TextBodyS), th.Color(theme.ColorTextSecondary))
	} else {
		drawText(&list, t.Shaper, bounds.Min.X+12, bounds.Min.Y+8, t.Value.Get(), th.TextStyle(theme.TextBodyS), th.Color(theme.ColorText))
	}
	if t.Assistive != "" {
		drawText(&list, t.Shaper, bounds.Min.X+12, bounds.Max.Y-8, t.Assistive, th.TextStyle(theme.TextLabelS), th.Color(theme.ColorTextSecondary))
	}
	return &list
}

func (t *TextInput) themeContext() theme.Context {
	if t.Theme != nil {
		return t.Theme
	}
	return theme.Default()
}

func replaceRunes(s string, start, end int, insert string) string {
	runes := []rune(s)
	start = clampInt(start, 0, len(runes))
	end = clampInt(end, 0, len(runes))
	if start > end {
		start, end = end, start
	}
	out := make([]rune, 0, len(runes)-end+start+utf8.RuneCountInString(insert))
	out = append(out, runes[:start]...)
	out = append(out, []rune(insert)...)
	out = append(out, runes[end:]...)
	return string(out)
}

func simpleTextLayout(textValue string, multiline bool) *text.TextLayout {
	runes := []rune(textValue)
	const (
		charWidth  = 8
		lineHeight = 18
	)
	lines := make([]text.ShapedLine, 0)
	addLine := func(lineText []rune, firstRune int, y float32) {
		lineWidth := float32(len(lineText)) * charWidth
		line := text.ShapedLine{
			Bounds:    text.RectFromXYWH(0, y, lineWidth, lineHeight),
			Baseline:  y + 14,
			FirstRune: firstRune,
			RuneCount: len(lineText),
		}
		for i := range lineText {
			x := float32(i) * charWidth
			line.Runs = append(line.Runs, text.GlyphRun{
				Glyphs:  []text.PositionedGlyph{{RuneIndex: firstRune + i}},
				Bounds:  text.RectFromXYWH(x, y, charWidth, lineHeight),
				Advance: charWidth,
				Text:    string(lineText[i : i+1]),
			})
		}
		lines = append(lines, line)
	}
	if multiline {
		start := 0
		lineStart := 0
		for i, r := range runes {
			if r == '\n' {
				addLine(runes[lineStart:i], start, float32(len(lines))*lineHeight)
				start += i - lineStart + 1
				lineStart = i + 1
			}
		}
		addLine(runes[lineStart:], start, float32(len(lines))*lineHeight)
	} else {
		addLine(runes, 0, 0)
	}
	if len(lines) == 0 {
		lines = append(lines, text.ShapedLine{Bounds: text.RectFromXYWH(0, 0, 0, lineHeight), Baseline: 14, FirstRune: 0, RuneCount: 0})
	}
	maxWidth := float32(0)
	for _, line := range lines {
		if w := line.Bounds.Width(); w > maxWidth {
			maxWidth = w
		}
	}
	return &text.TextLayout{
		Lines:      lines,
		Bounds:     text.RectFromXYWH(0, 0, maxWidth, float32(len(lines))*lineHeight),
		LineHeight: lineHeight,
		Baseline:   14,
	}
}

func textRectToGfx(r text.Rect) gfx.Rect {
	return gfx.RectFromXYWH(r.Min.X, r.Min.Y, r.Width(), r.Height())
}

func maxf(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
