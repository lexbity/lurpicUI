package theme

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/fontdata"
	"codeburg.org/lexbit/lurpicui/layout"
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
		ctx.Spacing(SpacingXS).Float32(),
		ctx.Spacing(SpacingS).Float32(),
		ctx.Spacing(SpacingM).Float32(),
		ctx.Spacing(SpacingL).Float32(),
		ctx.Spacing(SpacingXL).Float32(),
		ctx.Spacing(SpacingXXL).Float32(),
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
	got := Default().TextStyle(TextMonoM)
	if got.Family == "" {
		t.Fatalf("mono family should be non-empty")
	}
	if isGenericFamilyName(got.Family) {
		t.Fatalf("mono family should be concrete, got %q", got.Family)
	}
}

func TestDefault_radius_none_is_zero(t *testing.T) {
	if got := Default().Radius(RadiusNone); got != layout.ResolvedScalar(0) {
		t.Fatalf("radius none should be zero, got %v", got)
	}
}

func TestDefault_radius_tokens_increasing(t *testing.T) {
	ctx := Default()
	if !(ctx.Radius(RadiusS).Float32() < ctx.Radius(RadiusM).Float32() && ctx.Radius(RadiusM).Float32() < ctx.Radius(RadiusL).Float32()) {
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
	ctx := Default()
	if _, ok := any(ctx).(ResolvedContext); !ok {
		t.Fatalf("default context should be a resolved context, got %T", ctx)
	}
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

func TestFontRoleResolve_prefers_loaded_family(t *testing.T) {
	reg := fontdata.TestFontRegistry(t)
	role := FontRole{
		PreferredFamilies: []string{"Missing Family", "Noto Sans"},
		DefaultStyle:      text.TextStyle{Size: 14, Weight: text.WeightRegular},
	}
	got := role.Resolve(reg)
	if got.Family != "Noto Sans" {
		t.Fatalf("family = %q", got.Family)
	}
}

func TestFontRolesRejectGenericFamilies(t *testing.T) {
	role := FontRole{PreferredFamilies: []string{"sans-serif"}, DefaultStyle: text.TextStyle{Size: 14}}
	if err := role.Validate("UISans"); err == nil {
		t.Fatal("expected generic family validation failure")
	}
}


