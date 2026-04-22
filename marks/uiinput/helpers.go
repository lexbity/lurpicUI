package uiinput

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/internal/markutil"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
)

func registerDescriptor(d marks.Descriptor) {
	marks.RegisterDescriptor(d)
}

func ensureBase(base *facet.Facet) {
	if base == nil {
		return
	}
	if base.ID() == 0 {
		*base = facet.NewFacet()
	}
}

func invalidate(base *facet.Facet, flags facet.DirtyFlags, source string) {
	if base == nil {
		return
	}
	base.InvalidateWithSource(flags, source)
}

func syncLayout(layoutRole *facet.LayoutRole, bounds gfx.Rect) {
	markutil.SyncLayout(layoutRole, bounds)
}

func syncViewport(viewport *facet.ViewportRole, transform gfx.Transform) {
	markutil.SyncViewport(viewport, transform)
}

func fillColor(m theme.Material, fallback gfx.Color) gfx.Color {
	return markutil.FillColor(m, fallback)
}

func strokeColor(m theme.Material, fallback gfx.Color) gfx.Color {
	return markutil.StrokeColor(m, fallback)
}

func strokeStyle(stroke theme.MaterialStroke) gfx.StrokeStyle {
	return markutil.StrokeStyle(stroke)
}

func drawText(list *gfx.CommandList, shaper *text.Shaper, x, y float32, s string, style text.TextStyle, color gfx.Color) {
	markutil.DrawText(list, shaper, x, y, s, style, color)
}

func clampInt(v, min, max int) int {
	return markutil.ClampInt(v, min, max)
}

func clampFloat(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func buttonHeight() float32 {
	return markutil.RegularUIInputBaseline().Button.Height.Regular - 4
}

func checkboxSize() float32 {
	return markutil.RegularUIInputBaseline().Checkbox.Size.Regular + 10
}

func radioGroupSize() float32 {
	return markutil.RegularUIInputBaseline().RadioGroup.Size.Regular
}

func selectMinWidth() float32 {
	return markutil.RegularUIInputBaseline().Select.MinWidth.Regular
}

func selectPaddingY() float32 {
	return markutil.RegularUIInputBaseline().Select.PaddingY.Regular
}

func selectItemHeight() float32 {
	return buttonHeight() - selectPaddingY()
}

func textInputMinWidth() float32 {
	return markutil.RegularUIInputBaseline().TextInput.MinWidth.Regular
}

func textInputPaddingY() float32 {
	return markutil.RegularUIInputBaseline().TextInput.PaddingY.Regular
}

func textInputMultilineHeight() float32 {
	return textInputMinWidth() - 20
}

func sliderTrackThickness() float32 {
	return markutil.RegularUIInputBaseline().Slider.TrackThickness.Regular
}

func sliderThumbSize() float32 {
	return markutil.RegularUIInputBaseline().Slider.ThumbSize.Regular
}

func switchTrackWidth() float32 {
	return markutil.RegularUIInputBaseline().Switch.TrackWidth.Regular + 8
}

func switchTrackHeight() float32 {
	return markutil.RegularUIInputBaseline().Switch.TrackHeight.Regular + 8
}
