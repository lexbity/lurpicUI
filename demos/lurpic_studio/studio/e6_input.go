package studio

import (
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks/input"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
)

// playInputFamily is the Input playground: text_field, number_field,
// color_picker, and a standalone primitive icon. The text field's typed stream
// writes its Value store; the number field's steppers/keys write its Value
// store; the color picker's hue wheel and arrows write its Color store (the
// input family's distinctive behavior: the IME/write-back loop that lands user
// input in a store).
type playInputFamily struct {
	scroll *demoList

	field *input.TextField
	name  *store.ValueStore[string]

	number *input.NumberField
	amount *store.ValueStore[float64]

	picker *input.ColorPicker
	color  *store.ValueStore[gfx.Color]

	glyph *primitive.Icon
}

// newPlayInputFamily builds the Input family playground.
func newPlayInputFamily() *playInputFamily {
	f := &playInputFamily{
		name:   store.NewValueStore(""),
		amount: store.NewValueStore(12.0),
		color:  store.NewValueStore(gfx.ColorFromRGBA8(66, 133, 244, 255)),
	}

	f.field = input.NewTextField("Source name", uiinput.TextInputOutlined, f.name)
	f.number = input.NewNumberField("Reload after (s)", f.amount)
	f.picker = input.NewColorPicker("Series color", f.color)
	f.glyph = primitive.NewIcon(primitive.IconSVG(iconRealtime))

	f.scroll = newDemoList(listGap,
		playgroundCard("text_field — click and type", f.field),
		playgroundCard("number_field — click and step", f.number),
		playgroundCard("color_picker — drag or arrow", f.picker),
		playgroundCard("icon — a standalone vector glyph", f.glyph),
	)
	return f
}

// wire has nothing beyond the marks' own store bindings.
func (f *playInputFamily) wire() func() { return nil }

// Name returns the text field's Value store.
func (f *playInputFamily) Name() *store.ValueStore[string] { return f.name }

// Amount returns the number field's Value store.
func (f *playInputFamily) Amount() *store.ValueStore[float64] { return f.amount }

// Color returns the color picker's Color store.
func (f *playInputFamily) Color() *store.ValueStore[gfx.Color] { return f.color }
