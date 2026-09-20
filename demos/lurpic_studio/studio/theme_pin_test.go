package studio

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/theme"
)

// TestStudioTheme_pinnedTokens asserts RX-1 FR-18: the studio resolves exactly
// its pinned token set. StudioThemeContext() takes no platform/environment
// input (structurally impossible to read the OS), and the resolved colors are
// byte-identical to the pinned tokens.
func TestStudioTheme_pinnedTokens(t *testing.T) {
	ctx := StudioThemeContext()
	pinned := StudioTokens()
	checks := []struct {
		role theme.ColorToken
		want gfx.Color
	}{
		{theme.ColorBackground, pinned.Color.Background},
		{theme.ColorSurface, pinned.Color.Surface},
		{theme.ColorPrimary, pinned.Color.Primary},
	}
	for _, c := range checks {
		if got := ctx.Color(c.role); got != c.want {
			t.Fatalf("studio %v = %v, want the pinned token %v", c.role, got, c.want)
		}
	}
}

// TestStudioTheme_pinnedBlueFamily asserts the A-13 regression: the studio's
// palette is the pinned blue family, not an OS-derived palette (the live app
// once rendered an OS orange theme; FR-18 pins it blue under any desktop).
func TestStudioTheme_pinnedBlueFamily(t *testing.T) {
	primary := StudioThemeContext().Color(theme.ColorPrimary)
	if primary.B <= primary.R || primary.B <= primary.G {
		t.Fatalf("studio primary = %v, want the blue family (B dominant)", primary)
	}
}
