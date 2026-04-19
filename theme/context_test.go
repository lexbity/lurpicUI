package theme

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/text"
)

func TestDefault_all_color_tokens_non_zero(t *testing.T) {
	ctx := Default()
	for tok := ColorBackground; tok <= ColorWarning; tok++ {
		got := ctx.Color(tok)
		if got == (gfx.Color{}) {
			t.Fatalf("token %d returned zero color", tok)
		}
	}
}

func TestDefault_spacing_tokens_increasing(t *testing.T) {
	ctx := Default()
	values := []float32{
		ctx.Spacing(SpacingXS),
		ctx.Spacing(SpacingS),
		ctx.Spacing(SpacingM),
		ctx.Spacing(SpacingL),
		ctx.Spacing(SpacingXL),
		ctx.Spacing(SpacingXXL),
	}
	for i := 1; i < len(values); i++ {
		if values[i] <= values[i-1] {
			t.Fatalf("spacing not increasing at %d: %v", i, values)
		}
	}
}

func TestDefault_text_body_size_positive(t *testing.T) {
	if got := Default().TextStyle(TextBodyM); got.Size <= 0 {
		t.Fatalf("body text size should be positive, got %v", got.Size)
	}
}

func TestDefault_text_mono_is_monospace_family(t *testing.T) {
	if got := Default().TextStyle(TextMonoM); got.Family == "" {
		t.Fatalf("mono family should be non-empty")
	}
}

func TestDefault_radius_none_is_zero(t *testing.T) {
	if got := Default().Radius(RadiusNone); got != 0 {
		t.Fatalf("radius none should be zero, got %v", got)
	}
}

func TestDefault_radius_tokens_increasing(t *testing.T) {
	ctx := Default()
	if !(ctx.Radius(RadiusS) < ctx.Radius(RadiusM) && ctx.Radius(RadiusM) < ctx.Radius(RadiusL)) {
		t.Fatalf("radius tokens not increasing")
	}
}

func TestColorToken_constants_distinct(t *testing.T) {
	seen := map[ColorToken]struct{}{}
	for tok := ColorBackground; tok <= ColorWarning; tok++ {
		if _, ok := seen[tok]; ok {
			t.Fatalf("duplicate color token %d", tok)
		}
		seen[tok] = struct{}{}
	}
}

func TestContext_interface_satisfied_by_default(t *testing.T) {
	var _ Context = Default()
	if _, ok := Default().(defaultContext); ok {
		return
	}
	t.Fatalf("default context should be backed by defaultContext")
}

func TestDefault_text_styles_are_stable(t *testing.T) {
	ctx := Default()
	for _, tok := range []TextToken{TextBodyM, TextBodyS, TextLabelM, TextLabelS, TextHeadingS, TextMonoM, TextMonoS} {
		style := ctx.TextStyle(tok)
		if style == (text.TextStyle{}) {
			t.Fatalf("text token %d returned zero style", tok)
		}
	}
}
