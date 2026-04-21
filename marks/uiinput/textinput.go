package uiinput

import (
	"sync"
	"unicode/utf8"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/text"
	uirecipe "codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
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
func (t *TextInput) OnAttach(ctx facet.AttachContext) { t.syncRoles() }
func (t *TextInput) OnDetach() {}
func (t *TextInput) OnActivate() {}
func (t *TextInput) OnDeactivate() {}

func (t *TextInput) ensureInit() {
	t.once.Do(func() {
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
			Focusable: func() bool { return !t.Disabled },
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
	layout := t.currentLayout()
	if layout == nil || layout.Bounds.IsEmpty() {
		if t.Multiline {
			return gfx.RectFromXYWH(0, 0, 280, 120)
		}
		return gfx.RectFromXYWH(0, 0, 280, 36)
	}
	height := layout.Bounds.Height()
	if height < 20 {
		height = 20
	}
	return gfx.RectFromXYWH(0, 0, maxf(280, layout.Bounds.Width()+16), height+16)
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
	layout := t.currentLayout()
	if layout == nil {
		layout = simpleTextLayout("", t.Multiline)
	}
		local := e.Position
	switch e.Kind {
	case platform.PointerPress:
		pos := layout.HitTest(text.Point{X: local.X - 8, Y: local.Y - 8})
		t.cursor = clampInt(pos.Index, 0, layout.RuneCount())
		t.selection = text.TextRange{Start: t.cursor, End: t.cursor}
		t.dragAnchor = t.cursor
		t.dragging = true
		return true
	case platform.PointerMove:
		if t.dragging {
			pos := layout.HitTest(text.Point{X: local.X - 8, Y: local.Y - 8})
			t.cursor = clampInt(pos.Index, 0, layout.RuneCount())
			t.selection = text.TextRange{Start: t.dragAnchor, End: t.cursor}
			return true
		}
	case platform.PointerRelease:
		if t.dragging {
			pos := layout.HitTest(text.Point{X: local.X - 8, Y: local.Y - 8})
			t.cursor = clampInt(pos.Index, 0, layout.RuneCount())
			t.selection = text.TextRange{Start: t.dragAnchor, End: t.cursor}
			t.dragging = false
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
		return true
	case platform.KeyHome:
		t.cursor = 0
		if e.Modifiers&platform.ModShift != 0 {
			t.selection.End = 0
		} else {
			t.selection = text.TextRange{}
		}
		return true
	case platform.KeyEnd:
		t.cursor = len(runes)
		if e.Modifiers&platform.ModShift != 0 {
			t.selection.End = len(runes)
		} else {
			t.selection = text.TextRange{}
		}
		return true
	case platform.KeyBackspace:
		if t.ReadOnly {
			return true
		}
		if !sel.IsEmpty() {
			applyInsert("")
			return true
		}
		if cursor > 0 {
			textValue = replaceRunes(textValue, cursor-1, cursor, "")
			t.Value.Set(textValue)
			t.cursor = cursor - 1
			t.selection = text.TextRange{Start: t.cursor, End: t.cursor}
			t.rebuildLayout()
		}
		return true
	case platform.KeyEnter:
		if t.Multiline && !t.ReadOnly {
			applyInsert("\n")
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
		return true
	}
	selection := t.selection.Normalized()
	textValue := replaceRunes(t.Value.Get(), selection.Start, selection.End, e.Text)
	t.Value.Set(textValue)
	t.cursor = selection.Start + utf8.RuneCountInString(e.Text)
	t.selection = text.TextRange{Start: t.cursor, End: t.cursor}
	t.composing = ""
	t.rebuildLayout()
	return true
}

func (t *TextInput) project(ctx facet.ProjectionContext) *gfx.CommandList {
	slots, _ := uirecipe.ResolveTextInputRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()}, TextInputOutlined)
	layout := t.currentLayout()
	var list gfx.CommandList
	bounds := t.bounds()
	field := slots.Field.Resolve(t.state.interactionState(), theme.DefaultTokens())
	list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(fillColor(field, gfx.Color{R: 1, G: 1, B: 1, A: 1}))})
	if t.state.focused {
		focus := slots.FocusRing.Resolve(theme.StateFocused, theme.DefaultTokens())
		if len(focus.Strokes) > 0 {
			list.Add(gfx.StrokeRect{Rect: bounds.Inset(-2, -2), Stroke: strokeStyle(focus.Strokes[0]), Brush: gfx.SolidBrush(strokeColor(focus, gfx.Color{R: 0.3, G: 0.5, B: 1, A: 1}))})
		}
	}
	if layout != nil {
		origin := gfx.Point{X: bounds.Min.X + 8, Y: bounds.Min.Y + 8}
		if t.selection.IsEmpty() {
			if t.state.focused {
		caretRect := textRectToGfx(layout.CaretRect(text.TextPosition{Index: t.cursor})).Offset(origin.X, origin.Y)
		list.Add(gfx.FillRect{Rect: caretRect, Brush: gfx.SolidBrush(fillColor(slots.Caret.Resolve(theme.StateDefault, theme.DefaultTokens()), gfx.Color{A: 1}))})
		}
	} else {
			for _, rect := range layout.SelectionRects(t.selection) {
				list.Add(gfx.DrawSelectionRects{Rects: []gfx.Rect{textRectToGfx(rect).Offset(origin.X, origin.Y)}, Brush: gfx.SolidBrush(fillColor(slots.Selection.Resolve(theme.StateDefault, theme.DefaultTokens()), gfx.Color{R: 0.3, G: 0.5, B: 1, A: 0.2}))})
			}
		}
	}
	if t.Value.Get() == "" && t.Placeholder != "" {
		list.Add(gfx.DrawPoints{Points: []gfx.Point{{X: bounds.Min.X + 8, Y: bounds.Min.Y + 16}}, Radius: 1, Brush: gfx.SolidBrush(fillColor(slots.Placeholder.Resolve(theme.StateDefault, theme.DefaultTokens()), gfx.Color{A: 0.5}))})
	}
	if t.Assistive != "" {
		list.Add(gfx.DrawPoints{Points: []gfx.Point{{X: bounds.Min.X + 8, Y: bounds.Max.Y - 4}}, Radius: 1, Brush: gfx.SolidBrush(fillColor(slots.AssistiveText.Resolve(theme.StateDefault, theme.DefaultTokens()), gfx.Color{A: 0.5}))})
	}
	return &list
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
