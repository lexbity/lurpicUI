package theme

import (
	"math"
	"reflect"
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/text"
)

func TestDefaultTokens_completeness(t *testing.T) {
	tokens := DefaultTokens()

	colorType := reflect.TypeOf(ColorTokens{})
	colorValue := reflect.ValueOf(tokens.Color)
	for i := 0; i < colorType.NumField(); i++ {
		field := colorType.Field(i)
		switch field.Type {
		case reflect.TypeOf(gfx.Color{}):
			if got := colorValue.Field(i).Interface().(gfx.Color); got == (gfx.Color{}) {
				t.Fatalf("color field %s is zero", field.Name)
			}
		case reflect.TypeOf([]gfx.Color{}):
			if got := colorValue.Field(i).Interface().([]gfx.Color); len(got) < 8 {
				t.Fatalf("data palette too short: %d", len(got))
			}
		}
	}

	typographyType := reflect.TypeOf(TypographyTokens{})
	typographyValue := reflect.ValueOf(tokens.Typography)
	for i := 0; i < typographyType.NumField(); i++ {
		field := typographyType.Field(i)
		if field.Type != reflect.TypeOf(text.TextStyle{}) {
			continue
		}
		style := typographyValue.Field(i).Interface().(text.TextStyle)
		if style.Size <= 0 {
			t.Fatalf("text style %s has non-positive size %v", field.Name, style.Size)
		}
	}

	if !(tokens.Spacing.XXS < tokens.Spacing.XS &&
		tokens.Spacing.XS < tokens.Spacing.SM &&
		tokens.Spacing.SM < tokens.Spacing.MD &&
		tokens.Spacing.MD < tokens.Spacing.LG &&
		tokens.Spacing.LG < tokens.Spacing.XL &&
		tokens.Spacing.XL < tokens.Spacing.XXL) {
		t.Fatalf("spacing scale is not strictly ordered: %+v", tokens.Spacing)
	}

	if tokens.Spacing.TouchTarget < 44 {
		t.Fatalf("touch target must be at least 44, got %v", tokens.Spacing.TouchTarget)
	}

	if !(tokens.Motion.DurationInstant < tokens.Motion.DurationShort &&
		tokens.Motion.DurationShort < tokens.Motion.DurationMedium &&
		tokens.Motion.DurationMedium < tokens.Motion.DurationLong &&
		tokens.Motion.DurationLong < tokens.Motion.DurationXLong) {
		t.Fatalf("motion durations are not strictly ordered: %+v", tokens.Motion)
	}

	if len(tokens.Color.DataPalette) < 8 {
		t.Fatalf("data palette too short: %d", len(tokens.Color.DataPalette))
	}
	assertDataPaletteHueSeparation(t, tokens.Color.DataPalette, 20)
}

func TestDarkTokens_contrastAndLuminance(t *testing.T) {
	light := DefaultTokens()
	dark := DarkTokens()

	if luminance(dark.Color.Background) >= luminance(light.Color.Background) {
		t.Fatalf("dark background should be darker than light background")
	}

	if got := contrastRatio(dark.Color.OnBackground, dark.Color.Background); got < 4.5 {
		t.Fatalf("on-background contrast too low: %v", got)
	}
	if got := contrastRatio(dark.Color.OnSurface, dark.Color.Surface); got < 4.5 {
		t.Fatalf("on-surface contrast too low: %v", got)
	}
	if got := contrastRatio(dark.Color.OnPrimary, dark.Color.Primary); got < 4.5 {
		t.Fatalf("on-primary contrast too low: %v", got)
	}
}

func TestTokens_helpers(t *testing.T) {
	tokens := DefaultTokens()

	if got := tokens.ColorFor("background"); got != tokens.Color.Background {
		t.Fatalf("unexpected background color: %#v", got)
	}
	if got := tokens.TextStyleFor("body-medium"); got != tokens.Typography.BodyMedium {
		t.Fatalf("unexpected body-medium style: %#v", got)
	}
	if got := tokens.SpacingFor("touch-target"); got != tokens.Spacing.TouchTarget {
		t.Fatalf("unexpected touch-target spacing: %v", got)
	}

	assertPanicsContains(t, func() { _ = tokens.ColorFor("no-such-color") }, "no-such-color")
	assertPanicsContains(t, func() { _ = tokens.SpacingFor("no-such-spacing") }, "no-such-spacing")
}

func TestTokens_densityScaling(t *testing.T) {
	tokens := DefaultTokens()
	if got := tokens.Scale(12); got != 12 {
		t.Fatalf("comfortable scaling should be unchanged, got %v", got)
	}

	tokens.Density = DensityTokens{Mode: DensityCompact, Scale: 0.75}
	if got := tokens.Scale(12); got != 9 {
		t.Fatalf("compact scaling should be 0.75x, got %v", got)
	}

	tokens.Density = DensityTokens{Mode: DensityTouch, Scale: 1.25}
	if got := tokens.Scale(10); got < 44 {
		t.Fatalf("touch scaling should enforce minimum 44, got %v", got)
	}
}

func TestElevationTokens_scaling(t *testing.T) {
	tokens := DefaultTokens().Elevation

	if tokens.Level0.Width != 0 {
		t.Fatalf("level0 width should be zero, got %v", tokens.Level0.Width)
	}
	if !(tokens.Level1.BlurRadius < tokens.Level2.BlurRadius &&
		tokens.Level2.BlurRadius < tokens.Level3.BlurRadius &&
		tokens.Level3.BlurRadius < tokens.Level4.BlurRadius) {
		t.Fatalf("elevation blur values are not strictly increasing: %+v", tokens)
	}
}

func assertPanicsContains(t *testing.T, fn func(), want string) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected panic containing %q", want)
		}
		got, ok := r.(string)
		if !ok {
			t.Fatalf("expected string panic, got %T", r)
		}
		if !strings.Contains(got, want) {
			t.Fatalf("panic %q does not contain %q", got, want)
		}
	}()
	fn()
}

func luminance(c gfx.Color) float64 {
	r, g, b, _ := c.ToRGBA8()
	return 0.2126*srgbComponent(float64(r)) + 0.7152*srgbComponent(float64(g)) + 0.0722*srgbComponent(float64(b))
}

func contrastRatio(a, b gfx.Color) float64 {
	l1 := luminance(a)
	l2 := luminance(b)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

func srgbComponent(v float64) float64 {
	v /= 255
	if v <= 0.03928 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

func assertDataPaletteHueSeparation(t *testing.T, palette []gfx.Color, minDegrees float64) {
	t.Helper()
	hs := make([]float64, len(palette))
	for i, c := range palette {
		r, g, b, _ := c.ToRGBA8()
		hs[i] = hue(float64(r), float64(g), float64(b))
	}
	for i := range hs {
		for j := i + 1; j < len(hs); j++ {
			if hueDistance(hs[i], hs[j]) < minDegrees {
				t.Fatalf("palette hues too close at %d/%d: %v vs %v", i, j, hs[i], hs[j])
			}
		}
	}
}

func hue(r, g, b float64) float64 {
	r /= 255
	g /= 255
	b /= 255
	max := math.Max(r, math.Max(g, b))
	min := math.Min(r, math.Min(g, b))
	delta := max - min
	if delta == 0 {
		return 0
	}
	var h float64
	switch max {
	case r:
		h = math.Mod((g-b)/delta, 6)
	case g:
		h = ((b - r) / delta) + 2
	default:
		h = ((r - g) / delta) + 4
	}
	h *= 60
	if h < 0 {
		h += 360
	}
	return h
}

func hueDistance(a, b float64) float64 {
	diff := math.Abs(a - b)
	if diff > 180 {
		diff = 360 - diff
	}
	return diff
}
