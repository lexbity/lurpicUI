package templates

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
)

func TestDensityModeStringAndScale(t *testing.T) {
	if got := DensityCompact.String(); got != "compact" {
		t.Fatalf("compact string = %q", got)
	}
	if got := DensityRegular.String(); got != "regular" {
		t.Fatalf("regular string = %q", got)
	}
	if got := DensityTouchspread.String(); got != "touchspread" {
		t.Fatalf("touchspread string = %q", got)
	}
	if got := DensityCompact.TypographyScale(); got != 0.93 {
		t.Fatalf("compact scale = %v", got)
	}
	if got := DensityRegular.TypographyScale(); got != 1 {
		t.Fatalf("regular scale = %v", got)
	}
	if got := DensityTouchspread.TypographyScale(); got != 1.08 {
		t.Fatalf("touchspread scale = %v", got)
	}
}

func TestDensityTripletFor(t *testing.T) {
	triplet := DensityTriplet{Compact: 1, Regular: 2, Touchspread: 3}
	if got := triplet.For(DensityCompact); got != 1 {
		t.Fatalf("compact = %v", got)
	}
	if got := triplet.For(DensityRegular); got != 2 {
		t.Fatalf("regular = %v", got)
	}
	if got := triplet.For(DensityTouchspread); got != 3 {
		t.Fatalf("touchspread = %v", got)
	}
}

func TestChartInheritanceResolveFallsBackToRoot(t *testing.T) {
	root := ColorTokens{
		DataPalette: []gfx.Color{
			gfx.ColorFromRGBA8(1, 2, 3, 255),
			gfx.ColorFromRGBA8(4, 5, 6, 255),
		},
		AxisStrong: gfx.ColorFromRGBA8(7, 8, 9, 255),
		AxisSubtle: gfx.ColorFromRGBA8(10, 11, 12, 255),
		GridStrong: gfx.ColorFromRGBA8(13, 14, 15, 255),
		GridSubtle: gfx.ColorFromRGBA8(16, 17, 18, 255),
	}

	contract := ChartInheritance{
		DataPalette: []gfx.Color{gfx.ColorFromRGBA8(20, 21, 22, 255)},
		AxisStrong:  nil,
		GridSubtle: func() *gfx.Color {
			color := gfx.ColorFromRGBA8(23, 24, 25, 255)
			return &color
		}(),
	}
	resolved := contract.Resolve(root)

	if len(resolved.DataPalette) != 1 {
		t.Fatalf("data palette should prefer explicit override: %#v", resolved.DataPalette)
	}
	if resolved.AxisStrong != root.AxisStrong {
		t.Fatalf("axis strong should fall back to root")
	}
	if resolved.GridSubtle == root.GridSubtle {
		t.Fatalf("grid subtle should use explicit override")
	}
}

func TestTemplateThemeValidate(t *testing.T) {
	theme := DefaultTemplateTheme("notes")
	if err := theme.Validate(); err != nil {
		t.Fatalf("validate failed: %v", err)
	}
}
