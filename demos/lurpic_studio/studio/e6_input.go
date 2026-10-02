package studio

import (
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks/input"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/marks/structure"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
)

// playInputFamily is the Input playground: text_field, number_field,
// color_picker, and a standalone primitive icon. The text field's typed stream
// writes its Value store; the number field's steppers/keys write its Value
// store; the color picker's hue wheel and arrows write its Color store (the
// input family's distinctive behavior: the IME/write-back loop that lands user
// input in a store). Every mark owns its truth (RX-2 Q2) — the family holds
// no ceremony stores; the accessors expose each mark's live store.
type playInputFamily struct {
	scroll *structure.ScrollRegion

	field *input.TextField

	number *input.NumberField

	picker *input.ColorPicker

	glyph *primitive.Icon
}

// newPlayInputFamily builds the Input family playground.
func newPlayInputFamily() *playInputFamily {
	f := &playInputFamily{}

	f.field = input.NewTextField("Source name", uiinput.TextInputOutlined, nil)
	f.number = input.NewNumberField("Reload after (s)", nil)
	f.picker = input.NewColorPicker("Series color", nil)
	f.glyph = primitive.NewIcon(primitive.IconSVG(iconRealtime))

	f.scroll = newPlayScroll(listGap,
		playgroundCard("text_field — click and type", f.field),
		playgroundCard("number_field — click and step", f.number),
		playgroundCard("color_picker — drag or arrow", f.picker),
		playgroundCard("icon — a standalone vector glyph", f.glyph),
	)
	return f
}

// wire has nothing beyond the marks' own store bindings.
func (f *playInputFamily) wire() func() { return nil }

// Name returns the text field's live Value store.
func (f *playInputFamily) Name() *store.ValueStore[string] { return f.field.Store() }

// Amount returns the number field's live Value store.
func (f *playInputFamily) Amount() *store.ValueStore[float64] { return f.number.Store() }

// Color returns the color picker's live Color store.
func (f *playInputFamily) Color() *store.ValueStore[gfx.Color] { return f.picker.Store() }
